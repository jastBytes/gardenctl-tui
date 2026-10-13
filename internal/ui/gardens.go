package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
)

// gardensView lists the gardens from the gardenctl configuration.
type gardensView struct{ listBase }

func newGardensView(a *App) *gardensView {
	v := &gardensView{newListBase(a, column{title: "Garden"}, column{title: "Alias"}, column{title: "Kubeconfig", max: 60})}
	rows := make([]row, 0, len(a.opts.Gardens))
	for _, g := range a.opts.Gardens {
		rows = append(rows, row{key: g.Name, cells: []string{g.Name, g.Alias, g.Kubeconfig}, data: g})
	}
	v.tbl.setRows(rows)
	v.loaded = true
	return v
}

func (v *gardensView) title() string    { return "gardens" }
func (v *gardensView) init() tea.Cmd    { return v.app.applyRestore(v.vid, v.tbl, v.open) }
func (v *gardensView) refresh() tea.Cmd { return nil }
func (v *gardensView) hints() []hint {
	return []hint{{"enter", "projects"}, {"tab", "seeds"}, {"/", "filter"}}
}

func (v *gardensView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.tableKey(msg); handled {
		return true, cmd
	}
	r, ok := v.tbl.selected()
	if !ok {
		return false, nil
	}
	g := r.data.(config.Garden)
	switch msg.String() {
	case "enter", "right":
		return true, push(newProjectsView(v.app, g))
	case "tab":
		return true, push(newSeedsView(v.app, g))
	}
	return false, nil
}

func (v *gardensView) render(width, height int) string { return v.renderTable(width, height) }
