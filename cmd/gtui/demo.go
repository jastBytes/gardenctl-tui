package main

import (
	"time"

	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func demoLandscape() *fake.Client {
	c := fake.Landscape()
	c.Delay = 300 * time.Millisecond
	return c
}
