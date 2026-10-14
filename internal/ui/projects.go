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
			column{title: "Owner", max: 40}, column{title: "Purpose", max: 50}),
		garden: g,
	}
}

func (v *projectsView) title() string { return v.garden.Name }
func (v *projectsView) init() tea.Cmd { return v.refresh() }

func (v *projectsView) refresh() tea.Cmd {
	g, withShoots := v.garden, !v.noShoots
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
	if !ok || !v.finishLoad(m) {
		return nil
	}
	data := m.data.(projectsData)
	if data.forbidden {
		v.noShoots = true
	}
	rows := make([]row, 0, len(data.projects))
	for _, p := range data.projects {
		var count string
		var color lipgloss.TerminalColor
		if data.shoots != nil {
			count, color = shootCountCell(data.shoots[p.Namespace])
		}
		rows = append(rows, row{
			key:    p.Name,
			cells:  []string{p.Name, p.Namespace, p.Phase, count, p.Owner, p.Purpose},
			colors: []lipgloss.TerminalColor{nil, nil, nil, color},
			data:   p,
		})
	}
	v.tbl.setRows(rows)
	return v.app.applyRestore(v.vid, v.tbl, v.open)
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
