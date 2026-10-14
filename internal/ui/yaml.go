package ui

import (
	"context"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// yamlView shows the manifest of a shoot.
type yamlView struct {
	app     *App
	vid     int
	garden  config.Garden
	shoot   gardener.Shoot
	text    string
	err     error
	loading bool
	loaded  bool
	vp      viewport.Model
	ready   bool
}

func newYAMLView(a *App, g config.Garden, s gardener.Shoot) *yamlView {
	return &yamlView{app: a, vid: a.newID(), garden: g, shoot: s}
}

func (v *yamlView) id() int         { return v.vid }
func (v *yamlView) title() string   { return "yaml" }
func (v *yamlView) capturing() bool { return false }
func (v *yamlView) isLoading() bool { return v.loading }
func (v *yamlView) init() tea.Cmd   { return v.refresh() }
func (v *yamlView) hints() []hint {
	return []hint{{"↑/↓", "scroll"}, {"y", "copy"}, {"esc", "back"}}
}

func (v *yamlView) refresh() tea.Cmd {
	v.loading = true
	g, ns, name := v.garden, v.shoot.Namespace, v.shoot.Name
	return load(v.vid, func(ctx context.Context) (any, error) {
		c, err := v.app.clients.get(g)
		if err != nil {
			return nil, err
		}
		return c.GetShootYAML(ctx, ns, name)
	})
}

func (v *yamlView) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(loadedMsg); ok {
		v.loading = false
		v.err = m.err
		if m.err == nil {
			v.text = strings.TrimRight(string(m.data.([]byte)), "\n")
			v.loaded = true
		}
	}
	return nil
}

func (v *yamlView) key(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "y":
		if !v.loaded {
			return true, nil
		}
		return true, v.app.copyCmd("YAML of "+v.shoot.Name, v.text+"\n")
	case "g", "home":
		v.vp.GotoTop()
		return true, nil
	case "G", "end":
		v.vp.GotoBottom()
		return true, nil
	case "up", "k", "down", "j", "pgup", "pgdown", "ctrl+u", "ctrl+d":
		var cmd tea.Cmd
		v.vp, cmd = v.vp.Update(msg)
		return true, cmd
	}
	return false, nil
}

func (v *yamlView) render(width, height int) string {
	if !v.loaded {
		if v.err != nil {
			return styles.Error.Render("Could not load: " + v.err.Error())
		}
		return styles.Dim.Render("loading…")
	}
	if !v.ready || v.vp.Width != width || v.vp.Height != height {
		offset := v.vp.YOffset
		v.vp = viewport.New(width, height)
		v.vp.YOffset = offset
		v.ready = true
	}
	v.vp.SetContent(highlightYAML(v.text, width))
	return v.vp.View()
}

// yamlKey matches the key of a YAML line, including list item dashes.
var yamlKey = regexp.MustCompile(`^(\s*(?:- )*)([^\s#"'-][^:]*):(\s|$)`)

// highlightYAML colors the keys and cuts lines to width.
func highlightYAML(text string, width int) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		l = ansi.Truncate(l, width, "…")
		if m := yamlKey.FindStringSubmatchIndex(l); m != nil {
			l = l[:m[4]] + styles.YAMLKey.Render(l[m[4]:m[5]]) + l[m[5]:]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}
