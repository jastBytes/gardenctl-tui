// Package fake provides an in-memory gardener.Client for tests and demo mode.
package fake

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

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
	// Delay is applied to every call (demo mode).
	Delay time.Duration
	// Calls counts calls per method name.
	Calls map[string]int
}

var _ gardener.Client = &Client{}

func (c *Client) enter(ctx context.Context, method string) error {
	c.mu.Lock()
	if c.Calls == nil {
		c.Calls = map[string]int{}
	}
	c.Calls[method]++
	err, delay := c.Err, c.Delay
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
	for _, s := range c.Shoots {
		if s.Namespace == namespace && s.Name == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("shoot %s/%s not found", namespace, name)
}

func (c *Client) filter(keep func(gardener.Shoot) bool) []gardener.Shoot {
	var out []gardener.Shoot
	for _, s := range c.Shoots {
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
	s := gardener.Shoot{
		Name: name, Namespace: "garden-" + project, Project: project, Seed: seed,
		Version: version, Provider: "aws", Region: "eu-central-1", Purpose: "development",
		TechnicalID: "shoot--" + project + "--" + name, Status: st,
		LastOp:     gardener.LastOperation{Type: "Reconcile", State: "Succeeded", Progress: 100, Updated: epoch},
		Conditions: healthyConds(), Created: epoch,
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
			}),
			mkShoot("alpha", "web", "gcp-us1", "1.33.2", gardener.StatusProgressing, func(s *gardener.Shoot) {
				s.Provider, s.Region = "gcp", "us-east1"
				s.LastOp = gardener.LastOperation{Type: "Reconcile", State: "Processing", Progress: 63, Message: "Waiting for worker nodes", Updated: epoch}
				s.Conditions[3] = gardener.Condition{Type: "EveryNodeReady", Status: "Progressing", Reason: "NodesRollingUpdate", Message: "Worker group pool-a is rolling", Since: epoch}
			}),
			mkShoot("beta", "db", "aws-eu1", "1.31.9", gardener.StatusError, func(s *gardener.Shoot) {
				s.Purpose = "production"
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
