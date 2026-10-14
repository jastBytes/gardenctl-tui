package gardener

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type update struct {
	shoots []Shoot
	err    error
}

// startWatch runs WatchShoots in the background and returns its updates and result.
func startWatch(t *testing.T, c Client, scope ShootScope) (<-chan update, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	updates, done := make(chan update, 100), make(chan error, 1)
	go func() {
		done <- c.WatchShoots(ctx, scope, WatchOptions{Throttle: 5 * time.Millisecond, MaxBackoff: 10 * time.Millisecond},
			func(s []Shoot, err error) { updates <- update{s, err} })
	}()
	return updates, done
}

func next(t *testing.T, updates <-chan update) update {
	t.Helper()
	select {
	case u := <-updates:
		return u
	case <-time.After(3 * time.Second):
		t.Fatal("no update")
		return update{}
	}
}

func shootNames(shoots []Shoot) []string {
	var out []string
	for _, s := range shoots {
		out = append(out, s.Name)
	}
	return out
}

func TestWatchShoots(t *testing.T) {
	raw := fake.NewClientBuilder().WithScheme(Scheme()).
		WithObjects(shoot("garden-a", "s1", "seed-1"), shoot("garden-b", "s2", "seed-2")).
		WithIndex(&gardencorev1beta1.Shoot{}, SeedNameField, func(o client.Object) []string {
			return []string{*o.(*gardencorev1beta1.Shoot).Spec.SeedName}
		}).Build()
	// The fake ignores the field selector for watches: events are filtered by seed.
	updates, _ := startWatch(t, New(raw), ShootScope{Seed: "seed-1"})

	if u := next(t, updates); u.err != nil || len(u.shoots) != 1 {
		t.Fatalf("initial list: %+v", u)
	}
	time.Sleep(50 * time.Millisecond) // let the watch start; the fake does not replay from the resource version

	ctx := context.Background()
	if err := raw.Create(ctx, shoot("garden-a", "s3", "seed-1")); err != nil {
		t.Fatal(err)
	}
	if err := raw.Create(ctx, shoot("garden-b", "other-seed", "seed-2")); err != nil {
		t.Fatal(err)
	}
	u := next(t, updates)
	for !contains(shootNames(u.shoots), "s3") {
		u = next(t, updates)
	}
	if contains(shootNames(u.shoots), "other-seed") {
		t.Fatalf("shoot of another seed in the list: %v", shootNames(u.shoots))
	}

	var s1 gardencorev1beta1.Shoot
	if err := raw.Get(ctx, client.ObjectKey{Namespace: "garden-a", Name: "s1"}, &s1); err != nil {
		t.Fatal(err)
	}
	s1.Status.LastErrors = []gardencorev1beta1.LastError{{Description: "boom"}}
	if err := raw.Update(ctx, &s1); err != nil {
		t.Fatal(err)
	}
	if err := raw.Delete(ctx, shoot("garden-a", "s3", "seed-1")); err != nil {
		t.Fatal(err)
	}
	for {
		u = next(t, updates)
		names := shootNames(u.shoots)
		if !contains(names, "s3") && u.shoots[0].Status == StatusError {
			break
		}
	}
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func TestWatchShootsForbidden(t *testing.T) {
	raw := fake.NewClientBuilder().WithScheme(Scheme()).WithObjects(shoot("garden-a", "s1", "seed-1")).
		WithInterceptorFuncs(interceptor.Funcs{Watch: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) (watch.Interface, error) {
			return nil, apierrors.NewForbidden(schema.GroupResource{Resource: "shoots"}, "", errors.New("no watch"))
		}}).Build()
	updates, done := startWatch(t, New(raw), ShootScope{Namespace: "garden-a"})
	if u := next(t, updates); len(u.shoots) != 1 {
		t.Fatalf("initial list: %+v", u)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrWatchUnsupported) {
			t.Fatalf("err = %v, want ErrWatchUnsupported", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not give up")
	}
}

func TestWatchShootsResyncAndRetry(t *testing.T) {
	var watches, lists atomic.Int32
	raw := fake.NewClientBuilder().WithScheme(Scheme()).WithObjects(shoot("garden-a", "s1", "seed-1")).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
				if lists.Add(1) == 3 {
					return errors.New("connection refused")
				}
				return c.List(ctx, l, opts...)
			},
			Watch: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) (watch.Interface, error) {
				w := watch.NewFake()
				if watches.Add(1) <= 2 {
					go w.Error(&metav1.Status{Status: metav1.StatusFailure, Code: 410, Reason: metav1.StatusReasonExpired})
				}
				return w, nil
			},
		}).Build()
	updates, _ := startWatch(t, New(raw), ShootScope{})
	next(t, updates) // list 1, watch 1 expires
	next(t, updates) // list 2, watch 2 expires
	if u := next(t, updates); u.err == nil {
		t.Fatalf("list 3 should report its error, got %+v", u)
	}
	if u := next(t, updates); u.err != nil || len(u.shoots) != 1 {
		t.Fatalf("list 4 after backoff: %+v", u)
	}
}
