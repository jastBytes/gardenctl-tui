package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
)

// shootWatch streams the shoots of one scope into a view as loadedMsgs.
type shootWatch struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan loadedMsg
	// group identifies the watches of the favorites monitor.
	group string
}

// watching reports whether shoot lists use watches instead of polling.
func (a *App) watching() bool {
	return a.opts.Config.Watch && a.opts.Config.RefreshInterval.Duration > 0
}

// startWatch watches the shoots of scope for the view vid. The returned
// command delivers the first message; the view asks for each further one
// with next. When watching is not possible, a message with fallback set
// tells the view to poll instead.
func (a *App) startWatch(vid int, g config.Garden, scope gardener.ShootScope) (*shootWatch, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &shootWatch{ctx: ctx, cancel: cancel, ch: make(chan loadedMsg)}
	go func() {
		send := func(m loadedMsg) {
			m.viewID, m.stream = vid, w
			select {
			case w.ch <- m:
			case <-ctx.Done():
			}
		}
		c, err := a.clients.get(g)
		if err == nil {
			err = c.WatchShoots(ctx, scope, a.opts.WatchOptions, func(s []gardener.Shoot, err error) {
				send(loadedMsg{data: s, err: err})
			})
		}
		if err != nil {
			send(loadedMsg{fallback: true})
		}
	}()
	return w, w.next()
}

// next waits for the next update of the watch.
func (w *shootWatch) next() tea.Cmd {
	return func() tea.Msg {
		select {
		case m := <-w.ch:
			return m
		case <-w.ctx.Done():
			return nil
		}
	}
}

func (w *shootWatch) stop() {
	if w != nil {
		w.cancel()
	}
}
