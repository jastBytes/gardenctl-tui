// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

type clientFactory func(config.Garden) (gardener.Client, error)

// dump prints gardens, projects and shoots as plain text. Errors of a single
// garden are printed inline so that one unreachable landscape does not hide the rest.
func dump(ctx context.Context, w io.Writer, gardens []config.Garden, newClient clientFactory) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	defer tw.Flush()

	var failed int
	for _, g := range gardens {
		fmt.Fprintf(tw, "garden %s\n", g.DisplayName())
		c, err := newClient(g)
		if err != nil {
			failed++
			fmt.Fprintf(tw, "  error: %v\n", err)
			continue
		}
		projects, err := c.ListProjects(ctx)
		if err != nil {
			failed++
			fmt.Fprintf(tw, "  error: %v\n", err)
			continue
		}
		for _, p := range projects {
			fmt.Fprintf(tw, "  project %s\t(%s)\n", p.Name, p.Namespace)
			if p.Namespace == "" {
				continue
			}
			shoots, err := c.ListShoots(ctx, p.Namespace)
			if err != nil {
				fmt.Fprintf(tw, "    error: %v\n", err)
				continue
			}
			for _, s := range shoots {
				fmt.Fprintf(tw, "    shoot %s\tseed=%s\tv%s\t%s\t%s\n", s.Name, s.Seed, s.Version, s.Status, s.LastOp)
			}
		}
	}
	if failed > 0 && failed == len(gardens) {
		return fmt.Errorf("all %d garden(s) failed", failed)
	}
	return nil
}
