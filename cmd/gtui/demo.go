package main

import (
	"time"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/gardener/fake"
)

func demoLandscape() *fake.Client {
	c := fake.Landscape()
	c.Delay = 300 * time.Millisecond
	go func() {
		for range time.Tick(600 * time.Millisecond) {
			c.Update(advanceDemo)
		}
	}()
	return c
}

// advanceDemo lets the reconciliation of the "web" shoot progress, finish and
// start again, so that the demo shows live updates.
func advanceDemo(c *fake.Client) {
	for i := range c.Shoots {
		s := &c.Shoots[i]
		if s.Name != "web" {
			continue
		}
		// Replace, never modify, the conditions: readers hold copies of the slice.
		conds := append([]gardener.Condition(nil), s.Conditions...)
		switch {
		case s.LastOp.State != "Processing":
			s.LastOp.Progress++
			if s.LastOp.Progress < 108 { // stay green for a few ticks
				continue
			}
			s.LastOp.State, s.LastOp.Progress, s.LastOp.Message = "Processing", 10, "Waiting for worker nodes"
			s.Status = gardener.StatusProgressing
			conds[3].Status, conds[3].Reason = "Progressing", "NodesRollingUpdate"
		case s.LastOp.Progress >= 97:
			s.LastOp.State, s.LastOp.Progress, s.LastOp.Message = "Succeeded", 100, "Shoot cluster has been successfully reconciled."
			s.Status = gardener.StatusHealthy
			conds[3].Status, conds[3].Reason, conds[3].Message = "True", "EveryNodeReady", ""
		default:
			s.LastOp.Progress++
		}
		s.LastOp.Updated = time.Now()
		s.Conditions = conds
	}
}
