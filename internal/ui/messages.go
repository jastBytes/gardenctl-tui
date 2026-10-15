// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const requestTimeout = 30 * time.Second

// loadedMsg carries the result of an async load to the view with viewID.
type loadedMsg struct {
	viewID int
	data   any
	err    error
	// stream is set for updates of a watch.
	stream *shootWatch
	// fallback reports that the watch is not possible; the view polls instead.
	fallback bool
}

// statusMsg sets the status line.
type statusMsg struct {
	text string
	err  bool
}

// tickMsg triggers an auto-refresh. gen invalidates stale tick chains.
type tickMsg struct{ gen int }

// pushMsg pushes a view onto the navigation stack.
type pushMsg struct{ v view }

// load runs fn with a timeout and wraps the result for viewID.
func load(viewID int, fn func(ctx context.Context) (any, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		data, err := fn(ctx)
		return loadedMsg{viewID: viewID, data: data, err: err}
	}
}

func push(v view) tea.Cmd { return func() tea.Msg { return pushMsg{v} } }

func setStatus(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text, err: isErr} }
}
