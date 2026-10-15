// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGardenctlPath(t *testing.T) {
	t.Setenv("GCTL_HOME", "/opt/garden")
	t.Setenv("GCTL_CONFIG_NAME", "")
	p, err := GardenctlPath()
	if err != nil || p != "/opt/garden/gardenctl-v2.yaml" {
		t.Fatalf("got %q, %v", p, err)
	}
	t.Setenv("GCTL_CONFIG_NAME", "other")
	if p, _ = GardenctlPath(); p != "/opt/garden/other.yaml" {
		t.Fatalf("got %q", p)
	}
}

func TestLoadGardenctl(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := write(t, t.TempDir(), "gardenctl-v2.yaml", `
linkKubeconfig: false
gardens:
- identity: landscape-dev
  name: dev
  kubeconfig: ~/.garden/dev.yaml
  context: ctx
- identity: landscape-live
  kubeconfig: /abs/live.yaml
`)
	cfg, err := LoadGardenctl(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Gardens) != 2 {
		t.Fatalf("gardens: %+v", cfg.Gardens)
	}
	g := cfg.Gardens[0]
	if g.Name != "landscape-dev" || g.Alias != "dev" || g.Context != "ctx" ||
		g.Kubeconfig != filepath.Join(home, ".garden/dev.yaml") {
		t.Fatalf("garden: %+v", g)
	}
	if cfg.LinkKubeconfig == nil || *cfg.LinkKubeconfig {
		t.Fatal("linkKubeconfig not parsed")
	}
	if got, ok := cfg.Garden("dev"); !ok || got.Name != "landscape-dev" {
		t.Fatal("lookup by alias failed")
	}
	if _, ok := cfg.Garden("nope"); ok {
		t.Fatal("unexpected garden")
	}
	if g.DisplayName() != "landscape-dev (dev)" || cfg.Gardens[1].DisplayName() != "landscape-live" {
		t.Fatal("display name")
	}
}

func TestLoadGardenctlErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadGardenctl(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("expected error for missing file")
	}
	p := write(t, dir, "bad.yaml", "gardens:\n- kubeconfig: x\n")
	if _, err := LoadGardenctl(p); err == nil {
		t.Fatal("expected error for missing identity")
	}
}

func TestLoadGTUIDefaults(t *testing.T) {
	cfg, err := LoadGTUI(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tools) != 4 || cfg.RefreshInterval.Duration != 5*time.Second || cfg.Gardenctl != "gardenctl" {
		t.Fatalf("defaults: %+v", cfg)
	}
	if cfg.Tools[1].ControlPlaneKey != "K" {
		t.Fatalf("control plane key: %+v", cfg.Tools[1])
	}
}

func TestLoadGTUI(t *testing.T) {
	p := write(t, t.TempDir(), "gtui.yaml", `
refreshInterval: "0"
providerEnv: true
rememberLocation: true
newWindow: true
terminal: [kitty, "--"]
tools:
- key: x
  name: htop
  command: [htop]
- key: "9"
  name: k9s-cp
  controlPlaneKey: "("
  command: [k9s]
`)
	cfg, err := LoadGTUI(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshInterval.Duration != 0 || !cfg.ProviderEnv || !cfg.NewWindow || !cfg.RememberLocation ||
		len(cfg.Terminal) != 2 || cfg.Terminal[0] != "kitty" || len(cfg.Tools) != 2 {
		t.Fatalf("cfg: %+v", cfg)
	}
	if cfg.Tools[0].ControlPlaneKey != "X" || cfg.Tools[1].ControlPlaneKey != "(" {
		t.Fatalf("tools: %+v", cfg.Tools)
	}
}

func TestWatchOption(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadGTUI(write(t, dir, "default.yaml", "refreshInterval: 10s\n"))
	if err != nil || !cfg.Watch {
		t.Fatalf("watch should default to true: %v %+v", err, cfg)
	}
	if len(cfg.Notify) != 1 || cfg.Notify[0] != NotifyBell {
		t.Fatalf("notify should default to the bell: %v", cfg.Notify)
	}
	cfg, err = LoadGTUI(write(t, dir, "off.yaml", "watch: false\nnotify: []\nfavoritesOnStart: true\n"))
	if err != nil || cfg.Watch || len(cfg.Notify) != 0 || !cfg.FavoritesOnStart {
		t.Fatalf("options not applied: %v %+v", err, cfg)
	}
}

func TestLoadGTUIValidation(t *testing.T) {
	for name, content := range map[string]string{
		"reserved":  "tools:\n- {key: q, name: a, command: [a]}\n",
		"duplicate": "tools:\n- {key: a, name: a, command: [a]}\n- {key: a, name: b, command: [b]}\n",
		"cp-dup":    "tools:\n- {key: a, name: a, command: [a]}\n- {key: A, name: b, command: [b]}\n",
		"nocommand": "tools:\n- {key: a, name: a}\n",
		"badDur":    "refreshInterval: soon\n",
		"badNotify": "notify: [email]\n",
	} {
		t.Run(name, func(t *testing.T) {
			p := write(t, t.TempDir(), "gtui.yaml", content)
			if _, err := LoadGTUI(p); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
