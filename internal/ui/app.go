// Package ui contains the Bubble Tea models of gtui.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
	"github.com/jastbytes/gardenctl-tui/internal/ui/styles"
)

// view is one screen on the navigation stack.
type view interface {
	// id is unique per view instance and routes loadedMsgs.
	id() int
	// title is the breadcrumb segment.
	title() string
	// init starts the initial load.
	init() tea.Cmd
	// refresh reloads the data (manual "r" and auto-refresh).
	refresh() tea.Cmd
	// update handles non-key messages (e.g. loadedMsg) for this view.
	update(msg tea.Msg) tea.Cmd
	// key handles a key press of the top view and reports whether it was consumed.
	key(msg tea.KeyMsg) (bool, tea.Cmd)
	// render draws the body.
	render(width, height int) string
	// capturing is true while the view consumes all keys (e.g. filter input).
	capturing() bool
	// isLoading is true while a load is in flight.
	isLoading() bool
	// hints lists the most important keys for the status line.
	hints() []hint
}

// hint is a key/description pair for the status line.
type hint struct{ key, desc string }

// Options configure the App.
type Options struct {
	Gardens []config.Garden
	Clients ClientFactory
	Config  *config.GTUI
	// Now is used for relative times (tests inject a fixed clock).
	Now func() time.Time
	// Sessions prepares gardenctl sessions for launched tools (nil disables launching).
	Sessions *session.Manager
	// Exec hands the terminal to a command (default tea.Exec).
	Exec ExecFunc
	// Available reports whether a tool is installed (default launcher.Available).
	Available func(config.Tool) bool
	// State stores recent targets and favorites (default: in memory only).
	State *state.Store
}

// App is the root model: navigation stack, status line and auto-refresh.
type App struct {
	opts    Options
	clients *clients
	stack   []view
	nextID  int

	width, height int
	spinner       spinner.Model

	status    string
	statusErr bool

	autoRefresh bool
	tickGen     int

	// busy is set while a tool is being prepared or running.
	busy             string
	providerEnv      bool
	gardenctlVersion string

	state    *state.Store
	showHelp bool
}

// New creates the root model.
func New(opts Options) *App {
	if opts.Config == nil {
		opts.Config = config.DefaultGTUI()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	st := opts.State
	if st == nil {
		st, _ = state.Load("")
	}
	a := &App{
		state:       st,
		opts:        opts,
		clients:     newClients(opts.Clients),
		width:       100,
		height:      30,
		spinner:     sp,
		autoRefresh: opts.Config.RefreshInterval.Duration > 0,
		providerEnv: opts.Config.ProviderEnv,
	}
	a.stack = []view{newGardensView(a)}
	return a
}

func (a *App) newID() int {
	a.nextID++
	return a.nextID
}

func (a *App) top() view { return a.stack[len(a.stack)-1] }

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.top().init(), a.spinner.Tick, a.scheduleTick(), a.checkVersion())
}

func (a *App) scheduleTick() tea.Cmd {
	if !a.autoRefresh {
		return nil
	}
	gen := a.tickGen
	return tea.Tick(a.opts.Config.RefreshInterval.Duration, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case tickMsg:
		if msg.gen != a.tickGen || !a.autoRefresh {
			return a, nil
		}
		var cmd tea.Cmd
		if !a.top().capturing() && !a.top().isLoading() && a.busy == "" {
			cmd = a.top().refresh()
		}
		return a, tea.Batch(cmd, a.scheduleTick())

	case statusMsg:
		a.status, a.statusErr = msg.text, msg.err
		return a, nil

	case sessionReadyMsg, targetConfirmedMsg, toolExitedMsg, versionMsg:
		return a, a.handleLaunchMsg(msg)

	case popMsg:
		a.popView()
		return a, nil

	case pushMsg:
		a.stack = append(a.stack, msg.v)
		a.status = ""
		return a, msg.v.init()

	case loadedMsg:
		for _, v := range a.stack {
			if v.id() == msg.viewID {
				if msg.err != nil {
					a.status, a.statusErr = msg.err.Error(), true
				} else if a.statusErr {
					a.status, a.statusErr = "", false
				}
				return a, v.update(msg)
			}
		}
		return a, nil // view was popped meanwhile

	case tea.KeyMsg:
		return a, a.handleKey(msg)
	}
	return a, a.top().update(msg)
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}
	if a.showHelp {
		a.showHelp = false
		return nil
	}
	top := a.top()
	if top.capturing() {
		_, cmd := top.key(msg)
		return cmd
	}
	switch msg.String() {
	case "q":
		return tea.Quit
	case "?":
		a.showHelp = true
		return nil
	case "ctrl+r":
		if _, ok := top.(*recentView); ok {
			return nil
		}
		return push(newRecentView(a))
	case "p":
		a.providerEnv = !a.providerEnv
		if a.providerEnv {
			return setStatus("provider-env on: launched tools get cloud provider credentials", false)
		}
		return setStatus("provider-env off", false)
	case "r":
		a.status = ""
		return top.refresh()
	case "a":
		if a.opts.Config.RefreshInterval.Duration <= 0 {
			return setStatus("auto-refresh is disabled in gtui.yaml (refreshInterval)", false)
		}
		a.autoRefresh = !a.autoRefresh
		a.tickGen++
		state := "off"
		if a.autoRefresh {
			state = "every " + a.opts.Config.RefreshInterval.String()
		}
		return tea.Batch(setStatus("auto-refresh "+state, false), a.scheduleTick())
	}
	if handled, cmd := top.key(msg); handled {
		return cmd
	}
	switch msg.String() {
	case "esc", "h", "left", "backspace":
		a.popView()
	}
	return nil
}

func (a *App) popView() {
	if len(a.stack) > 1 {
		a.stack = a.stack[:len(a.stack)-1]
		a.status = ""
	}
}

func (a *App) View() string {
	w := max(20, a.width-2) // App style has horizontal padding of 1
	header := a.breadcrumb(w)
	status := a.statusLine(w)
	bodyHeight := max(3, a.height-lipgloss.Height(header)-lipgloss.Height(status)-1)
	var body string
	if a.showHelp {
		body = a.renderHelp(w, bodyHeight)
	} else {
		body = a.top().render(w, bodyHeight)
	}
	body = lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).Render(body)
	return styles.App.Render(lipgloss.JoinVertical(lipgloss.Left, header, body, status))
}

func (a *App) breadcrumb(width int) string {
	parts := make([]string, 0, len(a.stack))
	for i, v := range a.stack {
		if i == len(a.stack)-1 {
			parts = append(parts, styles.CrumbLast.Render(v.title()))
		} else {
			parts = append(parts, styles.Crumb.Render(v.title()))
		}
	}
	left := styles.Logo.Render("gtui") + "  " + strings.Join(parts, styles.Crumb.Render(" › "))
	right := styles.Dim.Render("auto-refresh off")
	if a.autoRefresh {
		right = styles.Dim.Render("⟳ " + a.opts.Config.RefreshInterval.String())
	}
	if a.providerEnv {
		right = styles.Info.Render("provider-env") + "  " + right
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left, width, "…")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (a *App) statusLine(width int) string {
	top := a.top()
	var left string
	switch {
	case a.busy != "":
		left = a.spinner.View() + " " + a.busy
	case top.isLoading():
		left = a.spinner.View() + " loading…"
	case a.status != "" && a.statusErr:
		left = styles.Error.Render("✗ " + a.status)
	case a.status != "":
		left = styles.Info.Render(a.status)
	}
	var hs []string
	for _, h := range append(top.hints(), hint{"?", "help"}, hint{"q", "quit"}) {
		hs = append(hs, fmt.Sprintf("%s %s", styles.Key.Render(h.key), styles.Dim.Render(h.desc)))
	}
	right := strings.Join(hs, styles.Dim.Render(" · "))
	if left == "" {
		return ansi.Truncate(right, width, "…")
	}
	if lipgloss.Width(left)+lipgloss.Width(right)+2 > width {
		return ansi.Truncate(left, width, "…")
	}
	return left + strings.Repeat(" ", width-lipgloss.Width(left)-lipgloss.Width(right)) + right
}
