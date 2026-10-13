package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/launcher"
	"github.com/jastbytes/gardenctl-tui/internal/session"
)

// envTimeout bounds `gardenctl kubectl-env` (targeting already happened in the terminal).
const envTimeout = time.Minute

// ExecFunc hands the terminal to a command (tea.Exec; replaced in tests).
type ExecFunc func(tea.ExecCommand, tea.ExecCallback) tea.Cmd

type launchReq struct {
	tool   config.Tool
	target session.Target
}

// sessionReadyMsg reports the result of preparing a session.
type sessionReadyMsg struct {
	req  launchReq
	sess *session.Session
	err  error
}

// targetedMsg reports the result of `gardenctl target` run in the terminal.
type targetedMsg struct {
	req    launchReq
	sess   *session.Session
	err    error
	stderr string
}

// toolExitedMsg is sent when a launched tool returns.
type toolExitedMsg struct {
	req launchReq
	err error
}

// versionMsg carries the result of the gardenctl version check.
type versionMsg struct {
	version string
	err     error
}

type popMsg struct{}

func pop() tea.Msg { return popMsg{} }

// toolForKey returns the tool bound to k and whether it targets the control plane.
func (a *App) toolForKey(k string, allowControlPlane bool) (config.Tool, bool, bool) {
	for _, t := range a.opts.Config.Tools {
		if t.Key == k {
			return t, false, true
		}
		if allowControlPlane && t.ControlPlaneKey == k {
			return t, true, true
		}
	}
	return config.Tool{}, false, false
}

// launchKey launches the tool bound to k against t, if any.
func (a *App) launchKey(k string, t session.Target) (bool, tea.Cmd) {
	tool, cp, ok := a.toolForKey(k, t.Shoot != "")
	if !ok {
		return false, nil
	}
	t.ControlPlane = cp
	return true, a.launch(tool, t)
}

func (a *App) available(tool config.Tool) bool {
	if a.opts.Available != nil {
		return a.opts.Available(tool)
	}
	return launcher.Available(tool)
}

// launch prepares a gardenctl session for t and then starts tool in it.
func (a *App) launch(tool config.Tool, t session.Target) tea.Cmd {
	if a.busy != "" {
		return setStatus("another launch is in progress", true)
	}
	if a.opts.Sessions == nil {
		return setStatus("launching is not available", true)
	}
	if !a.available(tool) {
		return setStatus(fmt.Sprintf("%s: %s not found in PATH", tool.Name, tool.Command[0]), true)
	}
	req := launchReq{tool: tool, target: t}
	a.busy = fmt.Sprintf("targeting %s…", t)
	a.status = ""
	sess := a.opts.Sessions.New(t)
	// Run in the terminal: gardenctl may ask for confirmation and login plugins
	// may print a URL. Keep a copy of stderr for the status line.
	var stderr strings.Builder
	e := &launcher.Exec{
		Cmd:    a.opts.Sessions.TargetCommand(sess),
		Banner: fmt.Sprintf("gtui: targeting %s", t),
		Stderr: &stderr,
	}
	return a.exec(e, func(err error) tea.Msg {
		return targetedMsg{req: req, sess: sess, err: err, stderr: stderr.String()}
	})
}

func (a *App) handleLaunchMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case targetedMsg:
		if msg.err != nil {
			a.busy = ""
			return setStatus(session.GardenctlError("target", msg.stderr, msg.err).Error(), true)
		}
		mgr := a.opts.Sessions
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), envTimeout)
			defer cancel()
			return sessionReadyMsg{req: msg.req, sess: msg.sess, err: mgr.Env(ctx, msg.sess)}
		}

	case sessionReadyMsg:
		if msg.err != nil {
			a.busy = ""
			return setStatus(msg.err.Error(), true)
		}
		return a.startTool(msg.req, msg.sess)

	case toolExitedMsg:
		a.busy = ""
		cmds := []tea.Cmd{a.top().refresh()}
		if msg.err != nil {
			cmds = append(cmds, setStatus(fmt.Sprintf("%s exited: %v", msg.req.tool.Name, msg.err), true))
		} else {
			cmds = append(cmds, setStatus(fmt.Sprintf("back from %s (%s)", msg.req.tool.Name, msg.req.target), false))
		}
		return tea.Batch(cmds...)

	case versionMsg:
		switch {
		case msg.err != nil:
			return setStatus(msg.err.Error()+" – launching tools will not work", true)
		case session.CompareVersions(msg.version, session.MinGardenctlVersion) < 0:
			return setStatus(fmt.Sprintf("gardenctl %s is older than %s – please upgrade", msg.version, session.MinGardenctlVersion), true)
		}
	}
	return nil
}

// toggleFavorite toggles t as favorite and reports the result in the status line.
func (a *App) toggleFavorite(t session.Target) tea.Cmd {
	t.ControlPlane = false
	on, err := a.state.ToggleFavorite(t)
	switch {
	case err != nil:
		return setStatus("saving favorites failed: "+err.Error(), true)
	case on:
		return setStatus("★ "+t.String()+" added to favorites (ctrl+r)", false)
	default:
		return setStatus(t.String()+" removed from favorites", false)
	}
}

func (a *App) startTool(req launchReq, sess *session.Session) tea.Cmd {
	cmd, err := launcher.Command(req.tool, sess, launcher.Options{
		ProviderEnv: a.providerEnv,
		Gardenctl:   a.opts.Config.Gardenctl,
	})
	if err != nil {
		a.busy = ""
		return setStatus(err.Error(), true)
	}
	var saveErr tea.Cmd
	if err := a.state.AddRecent(req.target, req.tool.Name); err != nil {
		saveErr = setStatus("saving recent targets failed: "+err.Error(), true)
	}
	if a.newWindow {
		a.busy = ""
		return tea.Batch(saveErr, a.startInWindow(req, sess, cmd))
	}
	a.busy = fmt.Sprintf("running %s on %s", req.tool.Name, req.target)
	banner := fmt.Sprintf("gtui: %s on %s – exit to return to gtui", req.tool.Name, req.target)
	return tea.Batch(saveErr, a.exec(&launcher.Exec{Cmd: cmd, Banner: banner}, func(err error) tea.Msg {
		return toolExitedMsg{req: req, err: err}
	}))
}

// startInWindow starts cmd in a new terminal window and returns right away;
// gtui stays usable while the tool runs.
func (a *App) startInWindow(req launchReq, sess *session.Session, cmd *exec.Cmd) tea.Cmd {
	terminal := a.opts.Config.Terminal
	if len(terminal) == 0 {
		terminal = launcher.DetectTerminal()
	}
	w, err := launcher.WindowCommand(cmd, terminal)
	if err != nil {
		return setStatus(err.Error(), true)
	}
	if err := w.Start(); err != nil {
		return setStatus(fmt.Sprintf("open window for %s: %v", req.tool.Name, err), true)
	}
	// The window may outlive gtui, so its session must survive the cleanup.
	a.opts.Sessions.Keep(sess)
	go func() { _ = w.Wait() }()
	return setStatus(fmt.Sprintf("started %s on %s in a new window", req.tool.Name, req.target), false)
}

func (a *App) exec(c tea.ExecCommand, cb tea.ExecCallback) tea.Cmd {
	if a.opts.Exec != nil {
		return a.opts.Exec(c, cb)
	}
	return tea.Exec(c, cb)
}

func (a *App) checkVersion() tea.Cmd {
	if a.opts.Sessions == nil {
		return nil
	}
	mgr := a.opts.Sessions
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		v, err := mgr.Version(ctx)
		return versionMsg{version: v, err: err}
	}
}
