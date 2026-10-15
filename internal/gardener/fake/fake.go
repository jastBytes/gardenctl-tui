// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Package fake provides an in-memory gardener.Client for tests and demo mode.
package fake

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// Client is an in-memory gardener.Client.
type Client struct {
	mu       sync.Mutex
	Projects []gardener.Project
	Seeds    []gardener.Seed
	Shoots   []gardener.Shoot
	// Err, if set, is returned by every call.
	Err error
	// MethodErr, if set for a method name, is returned by that method.
	MethodErr map[string]error
	// Delay is applied to every call (demo mode).
	Delay time.Duration
	// Calls counts calls per method name.
	Calls map[string]int

	watchers []chan struct{}
}

// Update changes the landscape and notifies running watches.
func (c *Client) Update(fn func(c *Client)) {
	c.mu.Lock()
	fn(c)
	watchers := c.watchers
	c.mu.Unlock()
	for _, w := range watchers {
		select {
		case w <- struct{}{}:
		default: // an update is already pending
		}
	}
}

// WatchShoots emits the shoots of scope initially and after every Update.
func (c *Client) WatchShoots(ctx context.Context, scope gardener.ShootScope, _ gardener.WatchOptions, emit func([]gardener.Shoot, error)) error {
	if err := c.enter(ctx, "WatchShoots"); err != nil {
		return fmt.Errorf("%w: %w", gardener.ErrWatchUnsupported, err)
	}
	changed := make(chan struct{}, 1)
	c.mu.Lock()
	c.watchers = append(c.watchers, changed)
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		for i, w := range c.watchers {
			if w == changed {
				c.watchers = append(c.watchers[:i], c.watchers[i+1:]...)
				break
			}
		}
	}()
	for {
		shoots := c.filter(func(s gardener.Shoot) bool {
			return (scope.Namespace == "" || s.Namespace == scope.Namespace) && (scope.Seed == "" || s.Seed == scope.Seed)
		})
		emit(shoots, nil)
		select {
		case <-ctx.Done():
			return nil
		case <-changed:
		}
	}
}

// Watchers returns the number of running watches.
func (c *Client) Watchers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.watchers)
}

var _ gardener.Client = &Client{}

func (c *Client) enter(ctx context.Context, method string) error {
	c.mu.Lock()
	if c.Calls == nil {
		c.Calls = map[string]int{}
	}
	c.Calls[method]++
	err, delay := c.Err, c.Delay
	if e := c.MethodErr[method]; e != nil {
		err = e
	}
	c.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// CallCount returns how often method was called.
func (c *Client) CallCount(method string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Calls[method]
}

func (c *Client) ListProjects(ctx context.Context) ([]gardener.Project, error) {
	if err := c.enter(ctx, "ListProjects"); err != nil {
		return nil, err
	}
	return append([]gardener.Project(nil), c.Projects...), nil
}

func (c *Client) ListShoots(ctx context.Context, namespace string) ([]gardener.Shoot, error) {
	if err := c.enter(ctx, "ListShoots"); err != nil {
		return nil, err
	}
	return c.filter(func(s gardener.Shoot) bool { return namespace == "" || s.Namespace == namespace }), nil
}

func (c *Client) ListShootsOnSeed(ctx context.Context, seed string) ([]gardener.Shoot, error) {
	if err := c.enter(ctx, "ListShootsOnSeed"); err != nil {
		return nil, err
	}
	return c.filter(func(s gardener.Shoot) bool { return s.Seed == seed }), nil
}

func (c *Client) ListSeeds(ctx context.Context) ([]gardener.Seed, error) {
	if err := c.enter(ctx, "ListSeeds"); err != nil {
		return nil, err
	}
	return append([]gardener.Seed(nil), c.Seeds...), nil
}

func (c *Client) GetShoot(ctx context.Context, namespace, name string) (*gardener.Shoot, error) {
	if err := c.enter(ctx, "GetShoot"); err != nil {
		return nil, err
	}
	for _, s := range c.shoots() {
		if s.Namespace == namespace && s.Name == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("shoot %s/%s not found", namespace, name)
}

// GetShootYAML renders a small manifest from the view model.
func (c *Client) GetShootYAML(ctx context.Context, namespace, name string) ([]byte, error) {
	if err := c.enter(ctx, "GetShootYAML"); err != nil {
		return nil, err
	}
	for _, s := range c.shoots() {
		if s.Namespace != namespace || s.Name != name {
			continue
		}
		m := map[string]any{
			"apiVersion": "core.gardener.cloud/v1beta1",
			"kind":       "Shoot",
			"metadata":   map[string]any{"name": s.Name, "namespace": s.Namespace},
			"spec": map[string]any{
				"kubernetes": map[string]any{"version": s.Version},
				"provider":   map[string]any{"type": s.Provider},
				"purpose":    s.Purpose,
				"region":     s.Region,
				"seedName":   s.Seed,
			},
			"status": map[string]any{"isHibernated": s.Hibernated, "technicalID": s.TechnicalID},
		}
		return yaml.Marshal(m)
	}
	return nil, fmt.Errorf("shoot %s/%s not found", namespace, name)
}

// shoots returns a copy of the shoots (Update may change them concurrently).
func (c *Client) shoots() []gardener.Shoot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]gardener.Shoot(nil), c.Shoots...)
}

func (c *Client) filter(keep func(gardener.Shoot) bool) []gardener.Shoot {
	var out []gardener.Shoot
	for _, s := range c.shoots() {
		if keep(s) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// epoch is a fixed timestamp so that rendered output is stable.
var epoch = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func healthyConds() []gardener.Condition {
	var out []gardener.Condition
	for _, t := range []string{"APIServerAvailable", "ControlPlaneHealthy", "ObservabilityComponentsHealthy", "EveryNodeReady", "SystemComponentsHealthy"} {
		out = append(out, gardener.Condition{Type: t, Status: "True", Reason: "HealthCheckSucceeded", Since: epoch})
	}
	return out
}

func mkShoot(project, name, seed, version string, st gardener.Status, mutate ...func(*gardener.Shoot)) gardener.Shoot {
	const region = "eu-central-1"
	s := gardener.Shoot{
		Name: name, Namespace: "garden-" + project, Project: project, Seed: seed,
		Version: version, Provider: "aws", Region: region, Purpose: "development",
		TechnicalID: "shoot--" + project + "--" + name, Status: st,
		LastOp:     gardener.LastOperation{Type: "Reconcile", State: "Succeeded", Progress: 100, Updated: epoch},
		Conditions: healthyConds(), Created: epoch,
		Domain:            name + "." + project + ".example.com",
		APIServer:         "https://api." + name + "." + project + ".example.com",
		Workers:           []gardener.Worker{{Name: "worker-a", MachineType: "m5.large", Image: "gardenlinux 1592.1.0", Min: 1, Max: 3, Zones: []string{region + "a"}}},
		MaintenanceWindow: "22:00-23:00 +0000",
		AutoUpdate:        []string{"Kubernetes", "machine images"},
		Rotations:         []gardener.Rotation{{Name: "Certificate authorities", Phase: "Completed", LastCompleted: epoch}},
	}
	for _, m := range mutate {
		m(&s)
	}
	return s
}

// Landscape returns a client with a small, deterministic demo landscape.
func Landscape() *Client {
	return &Client{
		Projects: []gardener.Project{
			{Name: "alpha", Namespace: "garden-alpha", Phase: "Ready", Purpose: "Team Alpha", Owner: "alice@example.com"},
			{Name: "beta", Namespace: "garden-beta", Phase: "Ready", Purpose: "Team Beta", Owner: "bob@example.com"},
			{Name: "ops", Namespace: "garden-ops", Phase: "Ready", Owner: "ops@example.com"},
		},
		Seeds: []gardener.Seed{
			{Name: "aws-eu1", Provider: "aws", Region: "eu-central-1", Status: gardener.StatusHealthy},
			{Name: "gcp-us1", Provider: "gcp", Region: "us-east1", Status: gardener.StatusProgressing},
		},
		Shoots: []gardener.Shoot{
			mkShoot("alpha", "api-gw", "aws-eu1", "1.33.2", gardener.StatusHealthy),
			mkShoot("alpha", "batch", "aws-eu1", "1.32.5", gardener.StatusHibernated, func(s *gardener.Shoot) {
				s.Hibernated = true
				s.HibernationSchedules = []string{"hibernate 00 20 * * 1-5, wake up 00 08 * * 1-5 (Europe/Berlin)"}
			}),
			mkShoot("alpha", "web", "gcp-us1", "1.33.2", gardener.StatusProgressing, func(s *gardener.Shoot) {
				s.Provider, s.Region = "gcp", "us-east1"
				s.LastOp = gardener.LastOperation{Type: "Reconcile", State: "Processing", Progress: 63, Message: "Waiting for worker nodes", Updated: epoch}
				s.Conditions[3] = gardener.Condition{Type: "EveryNodeReady", Status: "Progressing", Reason: "NodesRollingUpdate", Message: "Worker group pool-a is rolling", Since: epoch}
			}),
			mkShoot("beta", "db", "aws-eu1", "1.31.9", gardener.StatusError, func(s *gardener.Shoot) {
				s.Purpose = "production"
				s.HibernationSchedules = nil
				s.Workers = append(s.Workers, gardener.Worker{Name: "db", MachineType: "r5.xlarge", Image: "gardenlinux 1592.1.0", Min: 3, Max: 3, Zones: []string{"eu-central-1a", "eu-central-1b", "eu-central-1c"}})
				s.Rotations = append(s.Rotations, gardener.Rotation{Name: "ETCD encryption key", Phase: "Preparing"})
				s.LastOp = gardener.LastOperation{Type: "Reconcile", State: "Error", Progress: 89, Message: "Flow failed", Updated: epoch}
				s.Conditions[1] = gardener.Condition{Type: "ControlPlaneHealthy", Status: "False", Reason: "DeploymentUnhealthy", Message: "kube-controller-manager is not available", Since: epoch}
				s.LastErrors = []string{"task \"Waiting until shoot infrastructure has been reconciled\" failed: quota exceeded"}
				s.Constraints = []gardener.Condition{{Type: "HibernationPossible", Status: "True", Since: epoch}, {Type: "MaintenancePreconditionsSatisfied", Status: "False", Reason: "PDBBlocking", Message: "PodDisruptionBudget blocks drain", Since: epoch}}
			}),
			mkShoot("ops", "monitoring", "gcp-us1", "1.33.2", gardener.StatusHealthy, func(s *gardener.Shoot) {
				s.Provider, s.Region = "gcp", "us-east1"
			}),
		},
	}
}
