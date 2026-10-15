// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// recentView lists favorites and recently used targets with the live status
// of their shoots (see monitor).
type recentView struct{ listBase }

// groupShoots is the polled result for one monitor group.
type groupShoots struct {
	group  string
	shoots []gardener.Shoot
}

func newRecentView(a *App) *recentView {
	v := &recentView{newListBase(a, column{title: " "}, column{title: "Target"}, column{title: "Status"},
		column{title: "Last Operation", max: 40}, column{title: "Updated", drop: 1}, column{title: "Last Tool", drop: 2}, column{title: "Used", drop: 3})}
	v.rebuild()
	v.loaded = true
	return v
}

func (v *recentView) rebuild() {
	st := v.app.state
	mon := v.app.mon
	now := v.app.opts.Now()
	var rows []row
	seen := map[string]bool{}
	add := func(e state.Entry, fav bool) {
		star, colors := "", make([]lipgloss.TerminalColor, 3)
		if fav {
			star, colors[0] = "★", styles.ColorYellow
		}
		used := ""
		if !e.Used.IsZero() {
			used = ago(now, e.Used)
		}
		cells := []string{star, e.Key(), "", "", "", e.Tool, used}
		key := favoriteKey(e)
		s, ok := mon.shoots[key]
		if ok {
			cells[2], cells[3], cells[4] = styles.StatusText(s.Status), lastOpCell(s.LastOp), age(now, s.LastOp.Updated)
			colors[2] = styles.StatusColor(s.Status)
		}
		rows = append(rows, row{key: e.Key(), cells: cells, colors: colors, data: e, changed: ok && mon.changes.recent(key, now)})
	}
	for _, f := range st.Favorites() {
		seen[f.Key()] = true
		add(f, true)
	}
	for _, r := range st.Recent() {
		if seen[r.Key()] {
			// favorite: fill in tool and time of last use
			for i := range rows {
				if rows[i].key == r.Key() {
					rows[i].cells[5], rows[i].cells[6] = r.Tool, ago(now, r.Used)
				}
			}
			continue
		}
		add(r, false)
	}
	v.tbl.setRows(rows)
}

func (v *recentView) title() string { return "favorites" }
func (v *recentView) init() tea.Cmd { return v.load() }

// refresh retries failed favorite watches, or polls when watching is off.
func (v *recentView) refresh() tea.Cmd {
	if v.app.watching() {
		v.app.mon.failed = map[string]bool{}
		v.rebuild()
		return v.app.syncMonitor()
	}
	return v.load()
}

// load polls the shoots of the favorites and recent targets when they are
// not watched.
func (v *recentView) load() tea.Cmd {
	if v.app.watching() {
		return nil
	}
	var cmds []tea.Cmd
	for key, grp := range v.app.monitorGroups(true) {
		cmds = append(cmds, load(v.vid, func(ctx context.Context) (any, error) {
			c, err := v.app.clients.get(grp.garden)
			if err != nil {
				return nil, err
			}
			shoots, err := c.ListShoots(ctx, grp.namespace)
			return groupShoots{group: key, shoots: shoots}, err
		}))
	}
	return tea.Batch(cmds...)
}

func (v *recentView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok || m.err != nil {
		return nil
	}
	g := m.data.(groupShoots)
	return v.app.monitorApply(g.group, g.shoots)
}

func (v *recentView) hints() []hint {
	return append(toolHints(v.app), hint{"enter", "launch…"}, hint{"*", "favorite"}, hint{"y", "copy"}, hint{"esc", "back"})
}

func (v *recentView) selectedTarget() (session.Target, bool) {
	r, ok := v.tbl.selected()
	if !ok {
		return session.Target{}, false
	}
	return r.data.(state.Entry).Target(), true
}

func (v *recentView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if t, ok := v.selectedTarget(); ok && !v.tbl.filtering() {
		if handled, cmd := v.app.launchKey(msg.String(), t); handled {
			return true, cmd
		}
		switch msg.String() {
		case "*":
			cmd := v.app.toggleFavorite(t)
			v.rebuild()
			return true, cmd
		case "enter", "l":
			return true, push(newMenuView(v.app, t))
		case "y":
			techID := ""
			if s, ok := v.app.mon.shoots[favoriteKey(state.FromTarget(t))]; ok {
				techID = s.TechnicalID
			}
			return true, push(newYankView(v.app, t, techID))
		}
	}
	return v.tableKey(msg)
}

func (v *recentView) render(width, height int) string {
	if len(v.tbl.all) == 0 {
		return styles.Dim.Render("No favorites or recent targets yet. Mark a shoot with * as favorite, or launch a tool on one.")
	}
	return v.renderTable(width, height)
}
