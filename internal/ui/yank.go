package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

type yankItem struct{ key, what, value string }

// yankView offers values of a target to copy to the clipboard.
type yankView struct {
	listBase
	target session.Target
}

func newYankView(a *App, t session.Target, technicalID string) *yankView {
	v := &yankView{
		listBase: newListBase(a, column{title: "Key"}, column{title: "Copy"}, column{title: "Value"}),
		target:   t,
	}
	name := t.Shoot
	if name == "" {
		name = t.Seed
	}
	items := []yankItem{
		{"n", "name", name},
		{"f", "path", t.String()},
		{"t", "target command", a.opts.Config.Gardenctl + " " + strings.Join(t.TargetArgs(), " ")},
	}
	if technicalID != "" {
		items = append(items, yankItem{"i", "technical ID", technicalID})
	}
	rows := make([]row, 0, len(items))
	for _, it := range items {
		rows = append(rows, row{key: it.key, cells: []string{it.key, it.what, it.value}, data: it})
	}
	v.tbl.setRows(rows)
	v.loaded = true
	return v
}

func (v *yankView) title() string    { return "copy" }
func (v *yankView) init() tea.Cmd    { return nil }
func (v *yankView) refresh() tea.Cmd { return nil }
func (v *yankView) hints() []hint {
	return []hint{{"n/f/t/i", "copy"}, {"enter", "copy selected"}, {"esc", "back"}}
}

func (v *yankView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	if !v.tbl.filtering() {
		for _, r := range v.tbl.all {
			if r.key == k {
				return true, v.copy(r.data.(yankItem))
			}
		}
		if k == "enter" || k == "y" {
			if r, ok := v.tbl.selected(); ok {
				return true, v.copy(r.data.(yankItem))
			}
		}
	}
	return v.tableKey(msg)
}

func (v *yankView) copy(it yankItem) tea.Cmd {
	return tea.Sequence(pop, v.app.copyCmd(it.what, it.value))
}

func (v *yankView) render(width, height int) string {
	head := styles.Dim.Render("Copy to the clipboard from ") + styles.CrumbLast.Render(v.target.String()) + "\n\n"
	return head + v.renderTable(width, height-2)
}
