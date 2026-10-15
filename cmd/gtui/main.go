// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Command gtui is a terminal UI for navigating Gardener landscapes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
	"github.com/jastbytes/gardenctl-tui/internal/session"
	"github.com/jastbytes/gardenctl-tui/internal/state"
	"github.com/jastbytes/gardenctl-tui/internal/ui"
)

var version = "dev"

type options struct {
	gardenctlConfig string
	gtuiConfig      string
	garden          string
	dump            bool
	demo            bool
	version         bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gtui:", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (*options, error) {
	o := &options{}
	fs := flag.NewFlagSet("gtui", flag.ContinueOnError)
	fs.StringVar(&o.gardenctlConfig, "config", "", "gardenctl config file (default $GCTL_HOME/gardenctl-v2.yaml or ~/.garden/gardenctl-v2.yaml)")
	fs.StringVar(&o.gtuiConfig, "gtui-config", "", "gtui config file (default ~/.garden/gtui.yaml)")
	fs.StringVar(&o.garden, "garden", "", "only show this garden (identity or alias)")
	fs.BoolVar(&o.dump, "dump", false, "print gardens, projects and shoots as text and exit")
	fs.BoolVar(&o.demo, "demo", false, "use a built-in fake landscape (no cluster access)")
	fs.BoolVar(&o.version, "version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	return o, nil
}

func run(args []string) error {
	o, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if o.version {
		fmt.Println(version)
		return nil
	}

	gardens, newClient, err := loadGardens(o)
	if err != nil {
		return err
	}

	if o.dump {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return dump(ctx, os.Stdout, gardens, newClient)
	}

	gtuiPath := o.gtuiConfig
	if gtuiPath == "" {
		if gtuiPath, err = config.GTUIPath(); err != nil {
			return err
		}
	}
	gtuiCfg, err := config.LoadGTUI(gtuiPath)
	if err != nil {
		return err
	}

	store, err := loadState(o.demo)
	if err != nil {
		return err
	}

	sessions := session.NewManager(gtuiCfg.Gardenctl)
	defer func() {
		if cerr := sessions.Cleanup(); cerr != nil {
			fmt.Fprintln(os.Stderr, "gtui: cleanup of gardenctl sessions failed:", cerr)
		}
	}()

	var start *session.Target
	if gtuiCfg.StartAtTarget && !o.demo {
		start = shellTarget(sessions)
	}

	app := ui.New(ui.Options{
		Gardens:     gardens,
		Clients:     ui.ClientFactory(newClient),
		Config:      gtuiCfg,
		Sessions:    sessions,
		State:       store,
		StartTarget: start,
	})
	// No signal context here: a Ctrl+C inside a launched tool must not quit gtui.
	// Bubble Tea handles signals itself and ignores them while a tool runs.
	// Restore stderr right after the UI so that the messages below reach the terminal.
	restoreStderr := redirectStderr()
	_, err = tea.NewProgram(app, tea.WithAltScreen()).Run()
	restoreStderr()
	if serr := app.SaveLocation(); serr != nil {
		fmt.Fprintln(os.Stderr, "gtui: saving the location failed:", serr)
	}
	return err
}

// shellTarget returns the gardenctl target of the calling shell, or nil if it
// has none (or gardenctl cannot tell, e.g. without a session ID).
func shellTarget(sessions *session.Manager) *session.Target {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	t, ok, err := sessions.CurrentTarget(ctx)
	if err != nil || !ok {
		return nil
	}
	return &t
}

// loadState opens ~/.garden/gtui-state.yaml (recent targets, favorites, location).
// Demo mode keeps its state in memory.
func loadState(demo bool) (*state.Store, error) {
	if demo {
		return state.Load("")
	}
	dir, err := config.HomeDir()
	if err != nil {
		return nil, err
	}
	return state.Load(filepath.Join(dir, "gtui-state.yaml"))
}

// redirectStderr sends stderr output of libraries (klog, credential plugins)
// to a log file while the full-screen UI owns the terminal.
func redirectStderr() func() {
	f, err := os.CreateTemp("", "gtui-*.log")
	if err != nil {
		return func() {}
	}
	orig := os.Stderr
	os.Stderr = f
	return func() {
		os.Stderr = orig
		info, statErr := f.Stat()
		_ = f.Close()
		if statErr == nil && info.Size() == 0 {
			_ = os.Remove(f.Name())
		} else {
			fmt.Fprintf(orig, "gtui: diagnostic output was written to %s\n", f.Name())
		}
	}
}

func loadGardens(o *options) ([]config.Garden, clientFactory, error) {
	if o.demo {
		land := demoLandscape()
		return []config.Garden{{Name: "demo"}, {Name: "demo-live", Alias: "live"}},
			func(config.Garden) (gardener.Client, error) { return land, nil }, nil
	}
	path := o.gardenctlConfig
	if path == "" {
		var err error
		if path, err = config.GardenctlPath(); err != nil {
			return nil, nil, err
		}
	}
	cfg, err := config.LoadGardenctl(path)
	if err != nil {
		return nil, nil, err
	}
	gardens := cfg.Gardens
	if o.garden != "" {
		g, ok := cfg.Garden(o.garden)
		if !ok {
			return nil, nil, fmt.Errorf("garden %q not found in %s", o.garden, path)
		}
		gardens = []config.Garden{g}
	}
	if len(gardens) == 0 {
		return nil, nil, fmt.Errorf("no gardens configured in %s", path)
	}
	return gardens, gardener.NewForGarden, nil
}
