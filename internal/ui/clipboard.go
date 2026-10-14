package ui

import (
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// copyToClipboard puts s into the system clipboard (when a clipboard tool
// such as pbcopy, wl-copy or xclip is available) and sends it as an OSC 52
// sequence to the terminal, which also works over SSH and in tmux
// (with `set -g set-clipboard on`).
func copyToClipboard(s string) error {
	nativeErr := clipboard.WriteAll(s)
	oscErr := writeOSC52(s)
	if nativeErr != nil && oscErr != nil {
		return oscErr
	}
	return nil
}

// writeOSC52 writes to the controlling terminal directly: stderr is
// redirected to a log file while the UI runs.
func writeOSC52(s string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = tty.Close() }()
	_, err = osc52.New(s).WriteTo(tty)
	return err
}

// copyCmd copies value and reports it in the status line.
func (a *App) copyCmd(what, value string) tea.Cmd {
	cp := a.opts.Copy
	return func() tea.Msg {
		if err := cp(value); err != nil {
			return statusMsg{text: "copy failed: " + err.Error(), err: true}
		}
		if len(value) > 60 || strings.Contains(value, "\n") {
			return statusMsg{text: "copied " + what}
		}
		return statusMsg{text: "copied " + what + ": " + value}
	}
}
