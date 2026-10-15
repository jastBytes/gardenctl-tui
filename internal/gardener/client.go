// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"sort"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/jastbytes/gardenctl-tui/internal/config"
)

// SeedNameField is the shoot field used to select shoots by seed.
const SeedNameField = "spec.seedName"

// Client reads Gardener resources from one garden cluster.
type Client interface {
	ListProjects(ctx context.Context) ([]Project, error)
	// ListShoots lists the shoots of a project namespace ("" = all namespaces).
	ListShoots(ctx context.Context, namespace string) ([]Shoot, error)
	ListShootsOnSeed(ctx context.Context, seed string) ([]Shoot, error)
	ListSeeds(ctx context.Context) ([]Seed, error)
	GetShoot(ctx context.Context, namespace, name string) (*Shoot, error)
	// GetShootYAML returns the shoot manifest as YAML (without managed fields).
	GetShootYAML(ctx context.Context, namespace, name string) ([]byte, error)
	// WatchShoots keeps the shoots of scope up to date, see crClient.WatchShoots.
	WatchShoots(ctx context.Context, scope ShootScope, opts WatchOptions, emit func([]Shoot, error)) error
}

// Scheme contains the Gardener core API types.
func Scheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(gardencorev1beta1.AddToScheme(s))
	return s
}

type crClient struct {
	c client.WithWatch
}

// New wraps a controller-runtime client.
func New(c client.WithWatch) Client { return &crClient{c: c} }

// NewForGarden builds a client from the garden's kubeconfig (and context override).
func NewForGarden(g config.Garden) (Client, error) {
	loader := &clientcmd.ClientConfigLoadingRules{ExplicitPath: g.Kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if g.Context != "" {
		overrides.CurrentContext = g.Context
	}
	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("garden %s: %w", g.Name, err)
	}
	if restCfg.ExecProvider != nil {
		// The TUI owns the terminal; credential plugins must not read from it.
		restCfg.ExecProvider.StdinUnavailable = true
		restCfg.ExecProvider.StdinUnavailableMessage = "gtui runs a full-screen UI; log in with kubectl first"
	}
	c, err := client.NewWithWatch(restCfg, client.Options{Scheme: Scheme()})
	if err != nil {
		return nil, fmt.Errorf("garden %s: %w", g.Name, err)
	}
	return New(c), nil
}

func (c *crClient) ListProjects(ctx context.Context) ([]Project, error) {
	var list gardencorev1beta1.ProjectList
	if err := c.c.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	out := make([]Project, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ProjectFromAPI(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *crClient) ListShoots(ctx context.Context, namespace string) ([]Shoot, error) {
	var opts []client.ListOption
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}
	return c.listShoots(ctx, opts...)
}

func (c *crClient) ListShootsOnSeed(ctx context.Context, seed string) ([]Shoot, error) {
	return c.listShoots(ctx, client.MatchingFields{SeedNameField: seed})
}

func (c *crClient) listShoots(ctx context.Context, opts ...client.ListOption) ([]Shoot, error) {
	var list gardencorev1beta1.ShootList
	if err := c.c.List(ctx, &list, opts...); err != nil {
		return nil, fmt.Errorf("list shoots: %w", err)
	}
	out := make([]Shoot, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, ShootFromAPI(&list.Items[i]))
	}
	sortByNamespaceAndName(out)
	return out, nil
}

func (c *crClient) ListSeeds(ctx context.Context) ([]Seed, error) {
	var list gardencorev1beta1.SeedList
	if err := c.c.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("list seeds: %w", err)
	}
	out := make([]Seed, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, SeedFromAPI(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *crClient) GetShoot(ctx context.Context, namespace, name string) (*Shoot, error) {
	var s gardencorev1beta1.Shoot
	if err := c.c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &s); err != nil {
		return nil, fmt.Errorf("get shoot %s/%s: %w", namespace, name, err)
	}
	out := ShootFromAPI(&s)
	return &out, nil
}

func (c *crClient) GetShootYAML(ctx context.Context, namespace, name string) ([]byte, error) {
	var s gardencorev1beta1.Shoot
	if err := c.c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &s); err != nil {
		return nil, fmt.Errorf("get shoot %s/%s: %w", namespace, name, err)
	}
	// The typed client drops TypeMeta; managed fields and the last applied
	// configuration only add noise.
	s.APIVersion, s.Kind = gardencorev1beta1.SchemeGroupVersion.String(), "Shoot"
	s.ManagedFields = nil
	delete(s.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	return yaml.Marshal(&s)
}
