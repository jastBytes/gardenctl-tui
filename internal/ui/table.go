package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

type column struct {
	title string
	// max caps the auto width (0 = no cap).
	max int
}

type row struct {
	// key identifies the row across refreshes (keeps the cursor in place).
	key   string
	cells []string
	// colors optionally sets a foreground color per cell.
	colors []lipgloss.TerminalColor
	// data is the underlying item.
	data any
}

// table is a scrollable, fuzzy-filterable table with a cursor.
type table struct {
	columns []column
	all     []row
	visible []row
	cursor  int
	offset  int
	filter  textinput.Model
}

func newTable(cols ...column) *table {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "filter"
	return &table{columns: cols, filter: ti}
}

// setRows replaces the rows and keeps the cursor on the previously selected key.
func (t *table) setRows(rows []row) {
	prev := ""
	if r, ok := t.selected(); ok {
		prev = r.key
	}
	t.all = rows
	t.applyFilter()
	for i, r := range t.visible {
		if r.key == prev {
			t.cursor = i
			return
		}
	}
	t.clamp()
}

func (t *table) applyFilter() {
	q := strings.TrimSpace(t.filter.Value())
	if q == "" {
		t.visible = t.all
		t.clamp()
		return
	}
	src := make([]string, len(t.all))
	for i, r := range t.all {
		src[i] = ansi.Strip(strings.Join(r.cells, " "))
	}
	matches := fuzzy.Find(q, src)
	t.visible = make([]row, 0, len(matches))
	for _, m := range matches {
		t.visible = append(t.visible, t.all[m.Index])
	}
	t.cursor, t.offset = 0, 0
}

func (t *table) clamp() {
	if t.cursor >= len(t.visible) {
		t.cursor = len(t.visible) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func (t *table) selected() (row, bool) {
	if t.cursor < 0 || t.cursor >= len(t.visible) {
		return row{}, false
	}
	return t.visible[t.cursor], true
}

func (t *table) filtering() bool { return t.filter.Focused() }

func (t *table) filterActive() bool { return t.filter.Value() != "" }

func (t *table) startFilter() tea.Cmd { return t.filter.Focus() }

func (t *table) clearFilter() {
	t.filter.SetValue("")
	t.filter.Blur()
	t.applyFilter()
}

// update handles navigation and filter keys. It reports whether the key was consumed.
func (t *table) update(msg tea.KeyMsg) (bool, tea.Cmd) {
	if t.filter.Focused() {
		switch msg.String() {
		case "esc":
			t.clearFilter()
			return true, nil
		case "enter", "tab":
			t.filter.Blur()
			return true, nil
		case "up", "down", "ctrl+n", "ctrl+p":
			// allow moving while typing
		default:
			var cmd tea.Cmd
			before := t.filter.Value()
			t.filter, cmd = t.filter.Update(msg)
			if t.filter.Value() != before {
				t.applyFilter()
			}
			return true, cmd
		}
	}
	switch msg.String() {
	case "up", "k", "ctrl+p":
		t.move(-1)
	case "down", "j", "ctrl+n":
		t.move(1)
	case "pgup", "ctrl+b":
		t.move(-10)
	case "pgdown", "ctrl+f":
		t.move(10)
	case "home", "g":
		t.cursor = 0
	case "end", "G":
		t.cursor = len(t.visible) - 1
		t.clamp()
	default:
		return false, nil
	}
	return true, nil
}

func (t *table) move(d int) {
	t.cursor += d
	t.clamp()
}

func (t *table) widths(total int) []int {
	n := len(t.columns)
	w := make([]int, n)
	for i, c := range t.columns {
		w[i] = lipgloss.Width(c.title)
		for _, r := range t.all {
			if i < len(r.cells) {
				w[i] = max(w[i], lipgloss.Width(r.cells[i]))
			}
		}
		if c.max > 0 && w[i] > c.max {
			w[i] = c.max
		}
	}
	gap := 2 * (n - 1)
	// Shrink the widest column until everything fits.
	for sum(w)+gap > total {
		widest := 0
		for i := range w {
			if w[i] > w[widest] {
				widest = i
			}
		}
		if w[widest] <= 4 {
			break
		}
		w[widest]--
	}
	return w
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func cell(s string, w int, st lipgloss.Style) string {
	s = ansi.Truncate(s, w, "…")
	return st.Width(w).MaxWidth(w).Render(s)
}

// view renders the header and as many rows as fit into height.
func (t *table) view(width, height int) string {
	if height < 2 {
		height = 2
	}
	const gutter = 2 // cursor marker
	w := t.widths(width - gutter)
	var b strings.Builder

	hdr := make([]string, len(t.columns))
	for i, c := range t.columns {
		hdr[i] = cell(strings.ToUpper(c.title), w[i], styles.Header)
	}
	b.WriteString("  " + strings.Join(hdr, "  "))

	rowsHeight := height - 1
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+rowsHeight {
		t.offset = t.cursor - rowsHeight + 1
	}
	if t.offset > max(0, len(t.visible)-rowsHeight) {
		t.offset = max(0, len(t.visible)-rowsHeight)
	}

	end := min(len(t.visible), t.offset+rowsHeight)
	for i := t.offset; i < end; i++ {
		r := t.visible[i]
		base := styles.Cell
		if i == t.cursor {
			base = styles.Selected
		}
		cells := make([]string, len(t.columns))
		for c := range t.columns {
			text := ""
			if c < len(r.cells) {
				text = r.cells[c]
			}
			st := base
			if c < len(r.colors) && r.colors[c] != nil {
				st = st.Foreground(r.colors[c])
			}
			cells[c] = cell(text, w[c], st)
		}
		sep, marker := "  ", "  "
		if i == t.cursor {
			sep, marker = base.Render(sep), styles.Cursor.Render("▸ ")
		}
		b.WriteString("\n")
		b.WriteString(marker + strings.Join(cells, sep))
	}
	if len(t.visible) == 0 {
		b.WriteString("\n  ")
		if t.filterActive() {
			b.WriteString(styles.Dim.Render("no matches"))
		} else {
			b.WriteString(styles.Dim.Render("nothing here"))
		}
	}
	return b.String()
}

// filterView renders the filter line, or "" when no filter is in use.
func (t *table) filterView() string {
	if !t.filter.Focused() && t.filter.Value() == "" {
		return ""
	}
	if t.filter.Focused() {
		return t.filter.View()
	}
	return styles.Dim.Render("filter: " + t.filter.Value() + "  (esc to clear)")
}
