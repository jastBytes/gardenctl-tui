// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ShootScope selects the shoots of a watch: those of a project namespace, of
// a seed, or all shoots of the garden (both empty).
type ShootScope struct {
	Namespace string
	Seed      string
}

func (s ShootScope) listOptions() []client.ListOption {
	switch {
	case s.Namespace != "":
		return []client.ListOption{client.InNamespace(s.Namespace)}
	case s.Seed != "":
		return []client.ListOption{client.MatchingFields{SeedNameField: s.Seed}}
	}
	return nil
}

func (s ShootScope) matches(sh *gardencorev1beta1.Shoot) bool {
	switch {
	case s.Namespace != "":
		return sh.Namespace == s.Namespace
	case s.Seed != "":
		return sh.Spec.SeedName != nil && *sh.Spec.SeedName == s.Seed
	}
	return true
}

// WatchOptions tune WatchShoots.
type WatchOptions struct {
	// Throttle is the minimum time between two updates (default 1s).
	Throttle time.Duration
	// MaxBackoff caps the wait between reconnects after errors (default 30s).
	MaxBackoff time.Duration
}

// ErrWatchUnsupported wraps errors after which watching makes no sense
// (watch forbidden or not supported); callers should poll instead.
var ErrWatchUnsupported = errors.New("watch not possible")

// errResync asks for a new list (watch expired).
var errResync = errors.New("resync")

// WatchShoots lists the shoots of scope and keeps the list up to date with a
// watch. emit gets the complete, sorted list after the initial list and after
// changes (at most once per Throttle), or a transient error, after which the
// watch reconnects with backoff. It returns nil when ctx is done, or an error
// wrapping ErrWatchUnsupported.
func (c *crClient) WatchShoots(ctx context.Context, scope ShootScope, opts WatchOptions, emit func([]Shoot, error)) error {
	if opts.Throttle <= 0 {
		opts.Throttle = time.Second
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 30 * time.Second
	}
	backoff := time.Second
	fromCache := true
	for {
		listed, err := c.watchShoots(ctx, scope, fromCache, opts.Throttle, emit)
		if ctx.Err() != nil {
			return nil
		}
		if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
			return fmt.Errorf("%w: %w", ErrWatchUnsupported, err)
		}
		if listed {
			backoff = time.Second
		}
		// After an expired watch, the cache may be older than the expired
		// version; list consistently then (like client-go's reflector).
		fromCache = !errors.Is(err, errResync)
		if !fromCache {
			continue
		}
		emit(nil, err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, opts.MaxBackoff)
	}
}

// watchShoots runs one list+watch cycle. It reports whether the list succeeded.
// fromCache lists with resourceVersion "0", which the API server answers from
// its watch cache instead of etcd; the watch catches up on anything newer.
func (c *crClient) watchShoots(ctx context.Context, scope ShootScope, fromCache bool, throttle time.Duration,
	emit func([]Shoot, error)) (bool, error) {
	opts := scope.listOptions()
	if fromCache {
		opts = append(opts, &client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: "0"}})
	}
	var list gardencorev1beta1.ShootList
	if err := c.c.List(ctx, &list, opts...); err != nil {
		return false, fmt.Errorf("list shoots: %w", err)
	}
	items := map[string]Shoot{}
	for i := range list.Items {
		s := &list.Items[i]
		items[s.Namespace+"/"+s.Name] = ShootFromAPI(s)
	}
	emit(sortedShoots(items), nil)

	rv := list.ResourceVersion
	var pending <-chan time.Time
	for {
		w, err := c.c.Watch(ctx, &gardencorev1beta1.ShootList{}, append(scope.listOptions(),
			&client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: rv, AllowWatchBookmarks: true}})...)
		if err != nil {
			return true, fmt.Errorf("watch shoots: %w", err)
		}
		rv, pending, err = consume(ctx, w, scope, items, rv, pending, throttle, emit)
		w.Stop()
		if err != nil {
			if apierrors.IsGone(err) || apierrors.IsResourceExpired(err) {
				return true, errResync
			}
			return true, err
		}
		if ctx.Err() != nil {
			return true, nil
		}
		// The server closed the watch (timeout): continue from rv.
	}
}

// consume applies watch events to items until the watch ends. It returns the
// last seen resource version and a still pending (throttled) update.
func consume(ctx context.Context, w watch.Interface, scope ShootScope, items map[string]Shoot, rv string,
	pending <-chan time.Time, throttle time.Duration, emit func([]Shoot, error)) (string, <-chan time.Time, error) {
	for {
		select {
		case <-ctx.Done():
			return rv, pending, nil
		case <-pending:
			pending = nil
			emit(sortedShoots(items), nil)
		case ev, ok := <-w.ResultChan():
			if !ok {
				return rv, pending, nil
			}
			if ev.Type == watch.Error {
				return rv, pending, apierrors.FromObject(ev.Object)
			}
			s, ok := ev.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				continue
			}
			if s.ResourceVersion != "" {
				rv = s.ResourceVersion
			}
			key := s.Namespace + "/" + s.Name
			switch {
			case ev.Type == watch.Bookmark:
				continue
			case ev.Type == watch.Deleted || !scope.matches(s):
				// A shoot can also leave a seed's scope (control plane migration).
				if _, ok := items[key]; !ok {
					continue
				}
				delete(items, key)
			default:
				items[key] = ShootFromAPI(s)
			}
			if pending == nil {
				pending = time.After(throttle)
			}
		}
	}
}

func sortedShoots(items map[string]Shoot) []Shoot {
	out := make([]Shoot, 0, len(items))
	for _, s := range items {
		out = append(out, s)
	}
	sortByNamespaceAndName(out)
	return out
}

func sortByNamespaceAndName(out []Shoot) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
}
