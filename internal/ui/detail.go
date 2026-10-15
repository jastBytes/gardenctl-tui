// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

const labelWidth = 14 // keep in sync with styles.Label

// detailView shows the status of a single shoot.
type detailView struct {
	app     *App
	vid     int
	garden  config.Garden
	shoot   gardener.Shoot
	target  session.Target
	loading bool
	vp      viewport.Model
	ready   bool
}

func newDetailView(a *App, g config.Garden, s gardener.Shoot, t session.Target) *detailView {
	return &detailView{app: a, vid: a.newID(), garden: g, shoot: s, target: t}
}

func (v *detailView) id() int         { return v.vid }
func (v *detailView) title() string   { return v.shoot.Name }
func (v *detailView) capturing() bool { return false }
func (v *detailView) isLoading() bool { return v.loading }
func (v *detailView) init() tea.Cmd   { return nil }
func (v *detailView) hints() []hint {
	return append(toolHints(v.app), hint{"enter", "launch…"}, hint{"v", "yaml"}, hint{"y", "copy"}, hint{"*", "favorite"}, hint{"esc", "back"})
}

func (v *detailView) refresh() tea.Cmd {
	v.loading = true
	g, ns, name := v.garden, v.shoot.Namespace, v.shoot.Name
	return load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		return c.GetShoot(ctx, ns, name)
	})
}

func (v *detailView) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(loadedMsg); ok {
		v.loading = false
		if m.err == nil {
			v.shoot = *m.data.(*gardener.Shoot)
		}
	}
	return nil
}

func (v *detailView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	if handled, cmd := v.app.launchKey(msg.String(), v.target); handled {
		return true, cmd
	}
	switch msg.String() {
	case "enter", "l":
		return true, push(newMenuView(v.app, v.target))
	case "*":
		return true, v.app.toggleFavorite(v.target)
	case "y":
		return true, push(newYankView(v.app, v.target, v.shoot.TechnicalID))
	case "v":
		return true, push(newYAMLView(v.app, v.garden, v.shoot))
	case "up", "k", "down", "j", "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end", "g", "G":
		switch msg.String() {
		case "g", "home":
			v.vp.GotoTop()
		case "G", "end":
			v.vp.GotoBottom()
		default:
			var cmd tea.Cmd
			v.vp, cmd = v.vp.Update(msg)
			return true, cmd
		}
		return true, nil
	}
	return false, nil
}

func (v *detailView) render(width, height int) string {
	if !v.ready || v.vp.Width != width || v.vp.Height != height {
		offset := v.vp.YOffset
		v.vp = viewport.New(width, height)
		v.vp.YOffset = offset
		v.ready = true
	}
	v.vp.SetContent(v.content(width))
	return v.vp.View()
}

func (v *detailView) content(width int) string {
	s := v.shoot
	now := v.app.opts.Now()
	var b strings.Builder
	field := func(label, value string) {
		value = strings.ReplaceAll(value, "\n", "\n"+strings.Repeat(" ", labelWidth))
		b.WriteString(styles.Label.Render(label) + value + "\n")
	}
	status := lipgloss.NewStyle().Foreground(styles.StatusColor(s.Status)).Render(styles.StatusText(s.Status))
	field("Shoot", styles.CrumbLast.Render(s.Name))
	field("Project", s.Project+styles.Dim.Render("  ("+s.Namespace+")"))
	field("Status", status)
	field("Seed", s.Seed)
	field("Kubernetes", s.Version)
	field("Provider", strings.TrimSpace(s.Provider+" "+s.Region))
	if s.Purpose != "" {
		field("Purpose", s.Purpose)
	}
	if s.TechnicalID != "" {
		field("Technical ID", s.TechnicalID)
	}
	if s.Domain != "" {
		field("Domain", s.Domain)
	}
	if s.APIServer != "" {
		field("API server", s.APIServer)
	}
	if !s.Created.IsZero() {
		field("Created", ago(now, s.Created))
	}
	if s.Hibernated {
		field("Hibernated", "yes")
	}
	if s.Deleting {
		field("Deleting", styles.Error.Render("yes"))
	}

	b.WriteString(styles.Section.Render("Last Operation") + "\n")
	if s.LastOp.Type == "" {
		b.WriteString(styles.Dim.Render("none") + "\n")
	} else {
		field("Operation", s.LastOp.String())
		if !s.LastOp.Updated.IsZero() {
			field("Updated", ago(now, s.LastOp.Updated))
		}
		if s.LastOp.Message != "" {
			field("Message", wrap(s.LastOp.Message, width-labelWidth))
		}
	}

	if len(s.LastErrors) > 0 {
		b.WriteString(styles.Section.Render("Errors") + "\n")
		for _, e := range s.LastErrors {
			b.WriteString(styles.Error.Render(wrap("✗ "+e, width)) + "\n")
		}
	}

	writeConditions(&b, "Conditions", s.Conditions, now, width)
	if len(s.Constraints) > 0 {
		writeConditions(&b, "Constraints", s.Constraints, now, width)
	}
	writeWorkers(&b, s.Workers)

	if s.MaintenanceWindow != "" || len(s.AutoUpdate) > 0 || len(s.HibernationSchedules) > 0 {
		b.WriteString(styles.Section.Render("Maintenance & Hibernation") + "\n")
		if s.MaintenanceWindow != "" {
			field("Window", s.MaintenanceWindow)
		}
		auto := "off"
		if len(s.AutoUpdate) > 0 {
			auto = strings.Join(s.AutoUpdate, ", ")
		}
		field("Auto-update", auto)
		for i, sc := range s.HibernationSchedules {
			label := ""
			if i == 0 {
				label = "Schedule"
			}
			field(label, sc)
		}
	}

	if len(s.Rotations) > 0 {
		b.WriteString(styles.Section.Render("Credentials Rotation") + "\n")
		nameW := 0
		for _, r := range s.Rotations {
			nameW = max(nameW, len(r.Name))
		}
		for _, r := range s.Rotations {
			state := "completed " + ago(now, r.LastCompleted)
			switch {
			case r.Phase != "" && r.Phase != "Completed":
				state = styles.Warn.Render(strings.ToLower(r.Phase))
			case r.LastCompleted.IsZero():
				state = styles.Dim.Render("never completed")
			}
			fmt.Fprintf(&b, "%-*s  %s\n", nameW, r.Name, state)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeWorkers renders the worker pools as a small table.
func writeWorkers(b *strings.Builder, workers []gardener.Worker) {
	if len(workers) == 0 {
		return
	}
	b.WriteString(styles.Section.Render("Workers") + "\n")
	rows := [][]string{{"POOL", "MACHINE", "IMAGE", "NODES", "ZONES"}}
	for _, w := range workers {
		rows = append(rows, []string{w.Name, w.MachineType, w.Image, fmt.Sprintf("%d-%d", w.Min, w.Max), strings.Join(w.Zones, ",")})
	}
	widths := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			widths[i] = max(widths[i], len(c))
		}
	}
	for i, r := range rows {
		var line strings.Builder
		for j, c := range r {
			fmt.Fprintf(&line, "%-*s  ", widths[j], c)
		}
		text := strings.TrimRight(line.String(), " ")
		if i == 0 {
			text = styles.Header.Render(text)
		}
		b.WriteString(text + "\n")
	}
}

func writeConditions(b *strings.Builder, title string, conds []gardener.Condition, now time.Time, width int) {
	b.WriteString(styles.Section.Render(title) + "\n")
	if len(conds) == 0 {
		b.WriteString(styles.Dim.Render("none") + "\n")
		return
	}
	typeW := 0
	for _, c := range conds {
		typeW = max(typeW, len(c.Type))
	}
	for _, c := range conds {
		st := lipgloss.NewStyle().Foreground(styles.ConditionColor(c.Status))
		line := fmt.Sprintf("%s  %-*s  %-11s", st.Render(styles.ConditionSymbol(c.Status)), typeW, c.Type, st.Render(fmt.Sprintf("%-11s", c.Status)))
		if !c.Since.IsZero() {
			line += styles.Dim.Render(" " + ago(now, c.Since))
		}
		b.WriteString(line + "\n")
		if c.Status != "True" && (c.Reason != "" || c.Message != "") {
			msg := strings.TrimSpace(c.Reason + ": " + c.Message)
			msg = strings.TrimSuffix(strings.TrimPrefix(msg, ": "), ":")
			b.WriteString(styles.Dim.Render(wrap(msg, width-4, "   ")) + "\n")
		}
	}
}

// ago formats the time between t and now, e.g. "3h ago".
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// wrap word-wraps s to width; continuation lines get indent (the first line too, if given).
func wrap(s string, width int, indent ...string) string {
	ind := ""
	if len(indent) > 0 {
		ind = indent[0]
	}
	if width < 10 {
		width = 10
	}
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > width {
			lines = append(lines, line)
			line = ""
		}
		if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	for i := range lines {
		lines[i] = ind + lines[i]
	}
	return strings.Join(lines, "\n")
}
