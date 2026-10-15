// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/jastbytes/gardenctl-tui/internal/config"
)

// notifier returns the default Options.Notify for the configured kinds.
func notifier(kinds []string) func(title, body string) {
	return func(title, body string) {
		for _, k := range kinds {
			switch k {
			case config.NotifyBell:
				_ = writeTTY("\a")
			case config.NotifyDesktop:
				_ = desktopNotification(title, body)
			}
		}
	}
}

// writeTTY writes to the controlling terminal (stderr is redirected while the UI runs).
func writeTTY(s string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = tty.Close() }()
	_, err = tty.WriteString(s)
	return err
}

func desktopNotification(title, body string) error {
	if runtime.GOOS == "darwin" {
		script := "display notification " + strconv.Quote(body) + " with title " + strconv.Quote(title)
		return exec.Command("osascript", "-e", script).Run() // #nosec G204 -- fixed binary, quoted text
	}
	return exec.Command("notify-send", "--app-name=gtui", title, body).Run() // #nosec G204 -- fixed binary
}
