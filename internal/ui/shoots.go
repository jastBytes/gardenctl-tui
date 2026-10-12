package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// shootsView lists the shoots of a project or of a seed.
type shootsView struct {
	listBase
	garden  config.Garden
	project *gardener.Project
	seed    string
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

func (v *shootsView) title() string {
	if v.project != nil {
		return v.project.Name
	}
	return "seed " + v.seed
}

func (v *shootsView) init() tea.Cmd { return v.refresh() }

func (v *shootsView) refresh() tea.Cmd {
	g := v.garden
	return v.startLoad(load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		if v.project != nil {
			return c.ListShoots(ctx, v.project.Namespace)
		}
		return c.ListShootsOnSeed(ctx, v.seed)
	}))
}

func (v *shootsView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok || !v.finishLoad(m) {
		return nil
	}
	shoots := m.data.([]gardener.Shoot)
	rows := make([]row, 0, len(shoots))
	for _, s := range shoots {
		second := s.Seed
		if v.project == nil {
			second = s.Project
		}
		rows = append(rows, row{
			key:    s.Namespace + "/" + s.Name,
			cells:  []string{s.Name, second, s.Version, styles.StatusText(s.Status), s.LastOp.String(), s.Purpose},
			colors: []lipgloss.TerminalColor{nil, nil, nil, styles.StatusColor(s.Status)},
			data:   s,
		})
	}
	v.tbl.setRows(rows)
	return nil
}

// target returns the launch target for a shoot of this view.
func (v *shootsView) target(s gardener.Shoot) session.Target {
	t := session.Target{Garden: v.garden.Name, Shoot: s.Name, Namespace: s.Namespace}
	if v.project != nil {
		t.Project = v.project.Name
	} else {
		t.Seed = v.seed
	}
	return t
}

func (v *shootsView) hints() []hint { return baseHints() }

func (v *shootsView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	r, ok := v.tbl.selected()
	if !ok {
		return false, nil
	}
	s := r.data.(gardener.Shoot)
	switch msg.String() {
	case "enter", "right":
		return true, push(newDetailView(v.app, v.garden, s, v.target(s)))
	}
	return false, nil
}

func (v *shootsView) render(width, height int) string { return v.renderTable(width, height) }
