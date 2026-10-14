package ui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// shootsView lists the shoots of a project, of a seed, or of the whole garden.
type shootsView struct {
	listBase
	garden  config.Garden
	project *gardener.Project
	seed    string
	shoots  []gardener.Shoot

	// watch keeps shoots up to date; nil while polling.
	watch *shootWatch
	// pending holds the latest watched list while auto-refresh is paused.
	pending []gardener.Shoot
}

func newProjectShootsView(a *App, g config.Garden, p gardener.Project) *shootsView {
	return &shootsView{
		listBase: newListBase(a, column{title: "Shoot"}, column{title: "Seed"}, column{title: "Version"},
			column{title: "Status"}, column{title: "Last Operation", max: 40}, column{title: "Purpose"}),
		garden:  g,
		project: &p,
	}
}

func newSeedShootsView(a *App, g config.Garden, seed string) *shootsView {
	return &shootsView{
		listBase: newListBase(a, column{title: "Shoot"}, column{title: "Project"}, column{title: "Version"},
			column{title: "Status"}, column{title: "Last Operation", max: 40}, column{title: "Purpose"}),
		garden: g,
		seed:   seed,
	}
}

// newGardenShootsView lists all shoots of the garden (neither project nor seed set).
func newGardenShootsView(a *App, g config.Garden) *shootsView {
	return &shootsView{
		listBase: newListBase(a, column{title: "Shoot"}, column{title: "Project"}, column{title: "Seed"}, column{title: "Version"},
			column{title: "Status"}, column{title: "Last Operation", max: 40}, column{title: "Purpose"}),
		garden: g,
	}
}

func (v *shootsView) all() bool { return v.project == nil && v.seed == "" }

func (v *shootsView) title() string {
	switch {
	case v.project != nil:
		return v.project.Name
	case v.all():
		return "all shoots"
	}
	return "seed " + v.seed
}

func (v *shootsView) init() tea.Cmd {
	if !v.app.watching() {
		return v.refresh()
	}
	var cmd tea.Cmd
	v.watch, cmd = v.app.startWatch(v.vid, v.garden, v.scope())
	return v.startLoad(cmd)
}

func (v *shootsView) scope() gardener.ShootScope {
	if v.project != nil {
		return gardener.ShootScope{Namespace: v.project.Namespace}
	}
	return gardener.ShootScope{Seed: v.seed}
}

func (v *shootsView) live() bool { return v.watch != nil }

func (v *shootsView) close() { v.watch.stop() }

func (v *shootsView) refresh() tea.Cmd {
	if v.watch != nil {
		// Watched lists are current; only show what arrived while paused.
		if v.pending != nil {
			v.shoots, v.pending = v.pending, nil
			v.rebuild()
		}
		return nil
	}
	g := v.garden
	return v.startLoad(load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		switch {
		case v.project != nil:
			return c.ListShoots(ctx, v.project.Namespace)
		case v.all():
			return c.ListShoots(ctx, "")
		}
		return c.ListShootsOnSeed(ctx, v.seed)
	}))
}

func (v *shootsView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok {
		return nil
	}
	var next tea.Cmd
	if m.stream != nil {
		if m.stream != v.watch {
			return nil // a stopped watch
		}
		if m.fallback {
			v.watch.stop()
			v.watch = nil
			return v.refresh()
		}
		next = v.watch.next()
		if m.err == nil && v.loaded && !v.app.autoRefresh {
			v.pending = m.data.([]gardener.Shoot)
			return next
		}
	}
	if !v.finishLoad(m) {
		return next
	}
	v.shoots, v.pending = m.data.([]gardener.Shoot), nil
	v.rebuild()
	return tea.Batch(next, v.app.applyRestore(v.vid, v.tbl, v.open))
}

func (v *shootsView) rebuild() {
	rows := make([]row, 0, len(v.shoots))
	for _, s := range sortShoots(v.shoots, v.app.sortBy) {
		if v.app.problemsOnly && (s.Status == gardener.StatusHealthy || s.Status == gardener.StatusHibernated) {
			continue
		}
		second := s.Seed
		if v.seed != "" {
			second = s.Project
		}
		name := s.Name
		if v.app.state.IsFavorite(v.target(s)) {
			name = "★ " + name
		}
		cells := []string{name, second, s.Version, styles.StatusText(s.Status), s.LastOp.String(), s.Purpose}
		colors := []lipgloss.TerminalColor{nil, nil, nil, styles.StatusColor(s.Status)}
		if v.all() {
			cells = append([]string{name, s.Project}, cells[1:]...)
			colors = append([]lipgloss.TerminalColor{nil}, colors...)
		}
		rows = append(rows, row{key: s.Namespace + "/" + s.Name, cells: cells, colors: colors, data: s})
	}
	v.tbl.setRows(rows)
}

// target returns the launch target for a shoot of this view.
func (v *shootsView) target(s gardener.Shoot) session.Target {
	t := session.Target{Garden: v.garden.Name, Shoot: s.Name, Namespace: s.Namespace}
	switch {
	case v.project != nil:
		t.Project = v.project.Name
	case v.all():
		t.Project = s.Project
	default:
		t.Seed = v.seed
	}
	return t
}

func (v *shootsView) hints() []hint {
	return append(toolHints(v.app), hint{"l", "launch…"}, hint{"enter", "details"}, hint{"/", "filter"}, hint{"!", "problems"}, hint{"esc", "back"})
}

func (v *shootsView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	// Tool shortcuts take precedence over vim-style navigation (if a tool uses j/k/g/G).
	if r, ok := v.tbl.selected(); ok && !v.tbl.filtering() {
		if handled, cmd := v.app.launchKey(msg.String(), v.target(r.data.(gardener.Shoot))); handled {
			return true, cmd
		}
	}
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	switch msg.String() {
	case "!":
		v.app.problemsOnly = !v.app.problemsOnly
		v.app.rebuildShootViews()
		return true, nil
	case "o":
		v.app.sortBy = (v.app.sortBy + 1) % sortModes
		v.app.rebuildShootViews()
		return true, setStatus(v.app.sortBy.describe(), false)
	}
	r, ok := v.tbl.selected()
	if !ok {
		return false, nil
	}
	s := r.data.(gardener.Shoot)
	switch msg.String() {
	case "enter", "right":
		return true, push(newDetailView(v.app, v.garden, s, v.target(s)))
	case "l":
		return true, push(newMenuView(v.app, v.target(s)))
	case "y":
		return true, push(newYankView(v.app, v.target(s), s.TechnicalID))
	case "v":
		return true, push(newYAMLView(v.app, v.garden, s))
	case "*":
		cmd := v.app.toggleFavorite(v.target(s))
		v.rebuild()
		return true, cmd
	}
	return false, nil
}

// toolHints lists the tool shortcuts, e.g. "s/k/c/f tools".
func toolHints(a *App) []hint {
	var keys []string
	for _, t := range a.opts.Config.Tools {
		keys = append(keys, t.Key)
	}
	if len(keys) == 0 {
		return nil
	}
	return []hint{{strings.Join(keys, "/"), "tools"}}
}

// rebuildShootViews applies changed list options (filter, order) to all shoot lists.
func (a *App) rebuildShootViews() {
	for _, sv := range a.stack {
		if sv, ok := sv.(*shootsView); ok {
			sv.rebuild()
		}
	}
}

func (v *shootsView) render(width, height int) string {
	if !v.loaded {
		return v.renderTable(width, height)
	}
	hidden := len(v.shoots) - len(v.tbl.all)
	return shootSummary(v.shoots, hidden) + "\n" + v.renderTable(width, height-1)
}
