package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// projectsView lists the projects of one garden.
type projectsView struct {
	listBase
	garden config.Garden
}

func newProjectsView(a *App, g config.Garden) *projectsView {
	return &projectsView{
		listBase: newListBase(a,
			column{title: "Project"}, column{title: "Namespace"}, column{title: "Phase"},
			column{title: "Owner", max: 40}, column{title: "Purpose", max: 50}),
		garden: g,
	}
}

func (v *projectsView) title() string { return v.garden.Name }
func (v *projectsView) init() tea.Cmd { return v.refresh() }

func (v *projectsView) refresh() tea.Cmd {
	g := v.garden
	return v.startLoad(load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		return c.ListProjects(ctx)
	}))
}

func (v *projectsView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok || !v.finishLoad(m) {
		return nil
	}
	projects := m.data.([]gardener.Project)
	rows := make([]row, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, row{key: p.Name, cells: []string{p.Name, p.Namespace, p.Phase, p.Owner, p.Purpose}, data: p})
	}
	v.tbl.setRows(rows)
	return v.app.applyRestore(v.vid, v.tbl, v.open)
}

func (v *projectsView) hints() []hint {
	return append(baseHints(), hint{"tab", "seeds"})
}

func (v *projectsView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	if msg.String() == "tab" {
		return true, push(newSeedsView(v.app, v.garden))
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
