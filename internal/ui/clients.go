package ui

import (
	"sync"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// ClientFactory creates a Gardener client for a garden.
type ClientFactory func(config.Garden) (gardener.Client, error)

// clients lazily creates and caches one client per garden. Failures are not
// cached so that a later refresh can recover (e.g. after a re-login).
type clients struct {
	mu      sync.Mutex
	factory ClientFactory
	cache   map[string]gardener.Client
}

func newClients(f ClientFactory) *clients {
	return &clients{factory: f, cache: map[string]gardener.Client{}}
}

func (c *clients) get(g config.Garden) (gardener.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cl, ok := c.cache[g.Name]; ok {
		return cl, nil
	}
	cl, err := c.factory(g)
	if err != nil {
		return nil, err
	}
	c.cache[g.Name] = cl
	return cl, nil
}
