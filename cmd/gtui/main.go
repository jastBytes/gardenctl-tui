// Command gtui is a terminal UI for navigating Gardener landscapes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jastbytes/gardenctl-tui/internal/config"
	"github.com/jastbytes/gardenctl-tui/internal/gardener"
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if o.dump {
		return dump(ctx, os.Stdout, gardens, newClient)
	}

	_, err = tea.NewProgram(model{}, tea.WithAltScreen()).Run()
	return err
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
