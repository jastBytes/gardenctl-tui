// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// Tool is a program that can be launched against a target.
type Tool struct {
	// Key is the single-character shortcut, e.g. "s".
	Key string `json:"key"`
	// Name is shown in the launcher menu.
	Name string `json:"name"`
	// Command is the argv. Placeholders: $SHELL, {garden}, {project}, {shoot}, {namespace}.
	Command []string `json:"command"`
	// ControlPlaneKey optionally launches the tool against the shoot's control plane.
	// Defaults to the upper-case variant of Key.
	ControlPlaneKey string `json:"controlPlaneKey,omitempty"`
}

// GTUI is gtui's own configuration (~/.garden/gtui.yaml).
type GTUI struct {
	Tools []Tool `json:"tools"`
	// RefreshInterval for auto-refresh, e.g. "30s". "0" disables auto-refresh.
	RefreshInterval Duration `json:"refreshInterval"`
	// ProviderEnv evaluates `gardenctl provider-env` in launched tools by default.
	ProviderEnv bool `json:"providerEnv,omitempty"`
	// NewWindow starts launched tools in a new terminal window by default
	// instead of handing over gtui's terminal.
	NewWindow bool `json:"newWindow,omitempty"`
	// Terminal is the command prefix that opens a new window, e.g.
	// ["tmux", "new-window"] or ["kitty", "--"]; the tool's argv is appended.
	// Empty: detected from the environment.
	Terminal []string `json:"terminal,omitempty"`
	// Gardenctl is the gardenctl binary to call.
	Gardenctl string `json:"gardenctl,omitempty"`
	// RememberLocation reopens the view (and selection) gtui was quit in.
	RememberLocation bool `json:"rememberLocation,omitempty"`
	// Watch keeps the shoot lists up to date with watches instead of
	// reloading them every RefreshInterval.
	Watch bool `json:"watch"`
	// Notify says how gtui reports a favorite shoot that turns red:
	// "bell" (terminal bell) and/or "desktop" (notify-send / osascript).
	Notify []string `json:"notify"`
	// FavoritesOnStart opens the favorites view on start (if there are favorites).
	FavoritesOnStart bool `json:"favoritesOnStart,omitempty"`
	// StartAtTarget opens gtui at the target of the calling shell's gardenctl
	// session, if it has one.
	StartAtTarget bool `json:"startAtTarget"`
}

// Notification kinds for Notify.
const (
	NotifyBell    = "bell"
	NotifyDesktop = "desktop"
)

// Duration is a time.Duration that (un)marshals as a string like "30s".
type Duration struct{ time.Duration }

func (d Duration) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "0" || s == "" {
		d.Duration = 0
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// DefaultTools are used when gtui.yaml defines no tools.
func DefaultTools() []Tool {
	return []Tool{
		{Key: "s", Name: "Shell", Command: []string{"$SHELL"}},
		// Not "k": that moves the cursor up in every list (vim keys).
		{Key: "9", Name: "k9s", Command: []string{"k9s"}, ControlPlaneKey: "K"},
		{Key: "c", Name: "kubectl", Command: []string{"$SHELL", "-c", "kubectl get nodes; exec $SHELL"}},
		{Key: "f", Name: "sofka", Command: []string{"sofka"}},
	}
}

// DefaultGTUI returns the built-in configuration.
func DefaultGTUI() *GTUI {
	tools := DefaultTools()
	for i := range tools {
		tools[i].ControlPlaneKey = tools[i].CPKey()
	}
	return &GTUI{
		Tools:           tools,
		RefreshInterval: Duration{5 * time.Second},
		Gardenctl:       "gardenctl",
		Watch:           true,
		Notify:          []string{NotifyBell},
		StartAtTarget:   true,
	}
}

// GTUIPath returns the default location of gtui.yaml.
func GTUIPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gtui.yaml"), nil
}

// LoadGTUI reads gtui.yaml; a missing file yields the defaults.
func LoadGTUI(filename string) (*GTUI, error) {
	cfg := DefaultGTUI()
	buf, err := os.ReadFile(filename) // #nosec G304 -- user-provided config path
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.Tools = nil
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = DefaultTools()
	}
	if cfg.Gardenctl == "" {
		cfg.Gardenctl = "gardenctl"
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	return cfg, nil
}

// reservedKeys are used by the UI and cannot be bound to tools.
var reservedKeys = map[string]bool{
	"q": true, "r": true, "a": true, "l": true, "h": true, "p": true, "w": true,
	"o": true, "v": true, "y": true, "/": true, "?": true, "*": true, "!": true,
}

func (c *GTUI) validate() error {
	for _, n := range c.Notify {
		if n != NotifyBell && n != NotifyDesktop {
			return fmt.Errorf("notify: unknown kind %q (bell, desktop)", n)
		}
	}
	seen := map[string]string{}
	for i := range c.Tools {
		c.Tools[i].ControlPlaneKey = c.Tools[i].CPKey()
	}
	for _, t := range c.Tools {
		if len(t.Command) == 0 {
			return fmt.Errorf("tool %q has no command", t.Name)
		}
		for _, k := range []string{t.Key, t.ControlPlaneKey} {
			if k == "" {
				continue
			}
			if reservedKeys[k] {
				return fmt.Errorf("tool %q: key %q is reserved", t.Name, k)
			}
			if other, ok := seen[k]; ok {
				return fmt.Errorf("tool %q: key %q already used by %q", t.Name, k, other)
			}
			seen[k] = t.Name
		}
	}
	return nil
}

// CPKey returns the control-plane shortcut: ControlPlaneKey, or upper-case Key.
func (t Tool) CPKey() string {
	if t.ControlPlaneKey != "" {
		return t.ControlPlaneKey
	}
	if up := strings.ToUpper(t.Key); up != t.Key {
		return up
	}
	return ""
}
