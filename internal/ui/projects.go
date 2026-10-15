// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// projectsView lists the projects of one garden.
type projectsView struct {
	listBase
	garden config.Garden
	// noShoots is set when listing all shoots is forbidden; the shoot counts stay empty.
	noShoots bool
	// watch keeps shoots up to date for the counts; nil while they are polled.
	watch    *shootWatch
	projects []gardener.Project
	// shoots by project namespace; nil if unknown.
	shoots map[string][]gardener.Shoot
}

// projectsData is the result of a projects load.
type projectsData struct {
	projects []gardener.Project
	// shoots by project namespace; nil if they could not be listed.
	shoots    map[string][]gardener.Shoot
	forbidden bool
}

func newProjectsView(a *App, g config.Garden) *projectsView {
	return &projectsView{
		listBase: newListBase(a,
			column{title: "Project"}, column{title: "Namespace"}, column{title: "Phase"}, column{title: "Shoots"},
			column{title: "Owner", max: 40, drop: 1}, column{title: "Purpose", max: 50, drop: 2}),
		garden: g,
	}
}

func (v *projectsView) title() string { return v.garden.Name }
func (v *projectsView) init() tea.Cmd {
	if !v.app.watching() {
		return v.refresh()
	}
	var cmd tea.Cmd
	v.watch, cmd = v.app.startWatch(v.vid, v.garden, gardener.ShootScope{})
	return tea.Batch(cmd, v.refresh())
}

func (v *projectsView) close() { v.watch.stop() }

func (v *projectsView) refresh() tea.Cmd {
	g, withShoots := v.garden, !v.noShoots && v.watch == nil
	return v.startLoad(load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		projects, err := c.ListProjects(ctx)
		if err != nil {
			return nil, err
		}
		data := projectsData{projects: projects}
		if !withShoots {
			return data, nil
		}
		// The shoot counts are optional: users without cluster-wide read
		// access still get the project list.
		shoots, err := c.ListShoots(ctx, "")
		if err != nil {
			data.forbidden = apierrors.IsForbidden(err)
			return data, nil
		}
		data.shoots = map[string][]gardener.Shoot{}
		for _, s := range shoots {
			data.shoots[s.Namespace] = append(data.shoots[s.Namespace], s)
		}
		return data, nil
	}))
}

func (v *projectsView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok {
		return nil
	}
	if m.stream != nil {
		return v.watchUpdate(m)
	}
	if !v.finishLoad(m) {
		return nil
	}
	data := m.data.(projectsData)
	if data.forbidden {
		v.noShoots = true
	}
	v.projects = data.projects
	if data.shoots != nil {
		v.shoots = data.shoots
	}
	v.rebuild()
	return v.app.applyRestore(v.vid, v.tbl, v.open)
}

// watchUpdate applies a watched shoot list to the counts.
func (v *projectsView) watchUpdate(m loadedMsg) tea.Cmd {
	switch {
	case m.stream != v.watch:
		return nil
	case m.fallback:
		v.watch.stop()
		v.watch = nil
		return v.refresh()
	case m.err != nil:
		return v.watch.next()
	}
	v.shoots = map[string][]gardener.Shoot{}
	for _, s := range m.data.([]gardener.Shoot) {
		v.shoots[s.Namespace] = append(v.shoots[s.Namespace], s)
	}
	if v.loaded {
		v.rebuild()
	}
	return v.watch.next()
}

func (v *projectsView) rebuild() {
	rows := make([]row, 0, len(v.projects))
	for _, p := range v.projects {
		var count string
		var color lipgloss.TerminalColor
		if v.shoots != nil {
			count, color = shootCountCell(v.shoots[p.Namespace])
		}
		rows = append(rows, row{
			key:    p.Name,
			cells:  []string{p.Name, p.Namespace, p.Phase, count, p.Owner, p.Purpose},
			colors: []lipgloss.TerminalColor{nil, nil, nil, color},
			data:   p,
		})
	}
	v.tbl.setRows(rows)
}

func (v *projectsView) hints() []hint {
	return append(baseHints(), hint{"tab", "seeds"}, hint{"A", "all shoots"})
}

func (v *projectsView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	switch msg.String() {
	case "tab":
		return true, push(newSeedsView(v.app, v.garden))
	case "A":
		return true, push(newGardenShootsView(v.app, v.garden))
	}
	r, ok := v.tbl.selected()
	if !ok {
		return false, nil
	}
	switch msg.String() {
	case "enter", "right":
		p := r.data.(gardener.Project)
		if p.Namespace == "" {
			return true, setStatus("project "+p.Name+" has no namespace yet", true)
		}
		return true, push(newProjectShootsView(v.app, v.garden, p))
	}
	return false, nil
}

func (v *projectsView) render(width, height int) string { return v.renderTable(width, height) }
