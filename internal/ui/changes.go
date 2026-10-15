// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// highlightFor is how long a row stays marked after its status changed.
const highlightFor = 10 * time.Second

// changeTracker remembers statuses and when they last changed.
type changeTracker struct {
	prev    map[string]gardener.Status
	changed map[string]time.Time
}

func newChangeTracker() *changeTracker {
	return &changeTracker{prev: map[string]gardener.Status{}, changed: map[string]time.Time{}}
}

// observe records the status of key and reports whether it changed. The
// first status of a key is no change.
func (c *changeTracker) observe(key string, st gardener.Status, now time.Time) (gardener.Status, bool) {
	old, known := c.prev[key]
	c.prev[key] = st
	if !known || old == st {
		return old, false
	}
	c.changed[key] = now
	return old, true
}

// recent reports whether key changed within highlightFor.
func (c *changeTracker) recent(key string, now time.Time) bool {
	t, ok := c.changed[key]
	return ok && now.Sub(t) < highlightFor
}

// rebuildMsg asks the view with viewID to redraw its rows (e.g. when a
// highlight expires).
type rebuildMsg struct{ viewID int }

// rebuilder views can redraw their rows without reloading.
type rebuilder interface{ rebuild() }

// expireHighlight redraws the view vid once a highlight set now has expired.
func expireHighlight(vid int) tea.Cmd {
	return tea.Tick(highlightFor+100*time.Millisecond, func(time.Time) tea.Msg { return rebuildMsg{viewID: vid} })
}

// lastOpCell renders a running operation as progress, e.g. "Reconcile 63% ▰▰▰▰▰▱▱▱",
// and other operations as text.
func lastOpCell(o gardener.LastOperation) string {
	if o.State != "Processing" && o.State != "Pending" {
		return o.String()
	}
	const width = 8
	filled := min(width, max(0, int(o.Progress)*width/100))
	return fmt.Sprintf("%s %d%% %s%s", o.Type, o.Progress, strings.Repeat("▰", filled), strings.Repeat("▱", width-filled))
}

// age formats the time since t compactly, e.g. "3m", "2h", "5d" ("" if unknown).
func age(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
