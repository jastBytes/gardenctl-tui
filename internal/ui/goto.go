// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// gotoView asks for a technical ID and jumps to that shoot.
type gotoView struct {
	app   *App
	vid   int
	input textinput.Model
}

// gotoMsg reports the garden in which the shoot of a technical ID was found.
type gotoMsg struct {
	garden string
	shoot  gardener.Shoot
	// project is the project name from the technical ID.
	project string
}

func newGotoView(a *App) *gotoView {
	ti := textinput.New()
	ti.Prompt = "technical ID: "
	ti.Placeholder = "shoot--project--name"
	ti.Focus()
	return &gotoView{app: a, vid: a.newID(), input: ti}
}

func (v *gotoView) id() int                { return v.vid }
func (v *gotoView) title() string          { return "go to" }
func (v *gotoView) init() tea.Cmd          { return textinput.Blink }
func (v *gotoView) refresh() tea.Cmd       { return nil }
func (v *gotoView) update(tea.Msg) tea.Cmd { return nil }
func (v *gotoView) capturing() bool        { return true }
func (v *gotoView) isLoading() bool        { return false }
func (v *gotoView) hints() []hint {
	return []hint{{"enter", "go"}, {"esc", "cancel"}}
}

func (v *gotoView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return true, pop
	case "enter":
		id := strings.TrimSpace(v.input.Value())
		if id == "" {
			return true, pop
		}
		return true, tea.Sequence(pop, setStatus("looking for "+id+"…", false), v.app.findTechnicalID(id))
	}
	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return true, cmd
}

func (v *gotoView) render(width, height int) string {
	v.input.Width = max(10, width-lipgloss.Width(v.input.Prompt)-2)
	return v.input.View() + "\n\n" +
		styles.Dim.Render("Paste the technical ID of a shoot (e.g. from a seed namespace or an alert);") + "\n" +
		styles.Dim.Render("gtui looks for it in all gardens and opens the shoot's details.")
}

// ParseTechnicalID splits "shoot--<project>--<shoot>". Neither name may contain "--".
func ParseTechnicalID(id string) (project, shoot string, ok bool) {
	rest, ok := strings.CutPrefix(id, "shoot--")
	if !ok {
		return "", "", false
	}
	project, shoot, ok = strings.Cut(rest, "--")
	if !ok || project == "" || shoot == "" || strings.Contains(shoot, "--") {
		return "", "", false
	}
	return project, shoot, true
}

var errShootNotFound = errors.New("not found")

// findShoot looks up the shoot with the technical ID id in one garden.
func findShoot(ctx context.Context, c gardener.Client, project, name, id string) (*gardener.Shoot, error) {
	match := func(ns string) (*gardener.Shoot, error) {
		s, err := c.GetShoot(ctx, ns, name)
		if err != nil {
			return nil, err
		}
		if s.TechnicalID != "" && s.TechnicalID != id {
			return nil, errShootNotFound
		}
		return s, nil
	}
	ns := "garden-" + project
	if project == "garden" {
		ns = "garden"
	}
	s, err := match(ns)
	if err == nil {
		return s, nil
	}
	// A project may use another namespace than garden-<project>.
	projects, perr := c.ListProjects(ctx)
	if perr != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.Name == project && p.Namespace != "" && p.Namespace != ns {
			return match(p.Namespace)
		}
	}
	return nil, err
}

// findTechnicalID searches all gardens in parallel for the shoot of id. If it
// exists in several gardens, the first one of the configuration wins.
func (a *App) findTechnicalID(id string) tea.Cmd {
	project, name, ok := ParseTechnicalID(id)
	if !ok {
		return setStatus(fmt.Sprintf("%q is not a technical ID (shoot--<project>--<name>)", id), true)
	}
	gardens := a.opts.Gardens
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		found := make([]*gardener.Shoot, len(gardens))
		var wg sync.WaitGroup
		for i, g := range gardens {
			wg.Add(1)
			go func(i int, g config.Garden) {
				defer wg.Done()
				c, err := a.clients.get(g)
				if err != nil {
					return
				}
				found[i], _ = findShoot(ctx, c, project, name, id)
			}(i, g)
		}
		wg.Wait()
		for i, s := range found {
			if s != nil {
				return gotoMsg{garden: gardens[i].Name, shoot: *s, project: project}
			}
		}
		return statusMsg{text: "no shoot with technical ID " + id + " in any garden", err: true}
	}
}

// jumpTo replaces the stack with the path to the shoot's details.
func (a *App) jumpTo(msg gotoMsg) tea.Cmd {
	loc := a.targetLocation(session.Target{Garden: msg.garden, Project: msg.project, Shoot: msg.shoot.Name})
	if loc == nil {
		return nil
	}
	for len(a.stack) > 1 {
		a.popView()
	}
	a.status = ""
	a.restore, a.restoreView = loc, a.stack[0].id()
	return a.stack[0].init()
}
