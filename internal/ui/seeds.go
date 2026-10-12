package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// seedsView lists the seeds of a garden.
type seedsView struct {
	listBase
	garden config.Garden
}

func newSeedsView(a *App, g config.Garden) *seedsView {
	return &seedsView{
		listBase: newListBase(a, column{title: "Seed"}, column{title: "Provider"}, column{title: "Region"}, column{title: "Status"}),
		garden:   g,
	}
}

func (v *seedsView) title() string { return v.garden.Name + " seeds" }
func (v *seedsView) init() tea.Cmd { return v.refresh() }

func (v *seedsView) refresh() tea.Cmd {
	g := v.garden
	return v.startLoad(load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		return c.ListSeeds(ctx)
	}))
}

func (v *seedsView) update(msg tea.Msg) tea.Cmd {
	m, ok := msg.(loadedMsg)
	if !ok || !v.finishLoad(m) {
		return nil
	}
	seeds := m.data.([]gardener.Seed)
	rows := make([]row, 0, len(seeds))
	for _, s := range seeds {
		rows = append(rows, row{
			key:    s.Name,
			cells:  []string{s.Name, s.Provider, s.Region, styles.StatusText(s.Status)},
			colors: []lipgloss.TerminalColor{nil, nil, nil, styles.StatusColor(s.Status)},
			data:   s,
		})
	}
	v.tbl.setRows(rows)
	return nil
}

func (v *seedsView) hints() []hint { return baseHints() }

func (v *seedsView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	r, ok := v.tbl.selected()
	if !ok {
		return false, nil
	}
	switch msg.String() {
	case "enter", "right":
		return true, push(newSeedShootsView(v.app, v.garden, r.data.(gardener.Seed).Name))
	}
	return false, nil
}

func (v *seedsView) render(width, height int) string { return v.renderTable(width, height) }
