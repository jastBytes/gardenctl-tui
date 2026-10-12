// Package config reads the gardenctl configuration and gtui's own configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	envHome       = "GCTL_HOME"
	envConfigName = "GCTL_CONFIG_NAME"

	defaultConfigName = "gardenctl-v2"
	homeFolder        = ".garden"
)

// Garden is one garden cluster from the gardenctl configuration.
type Garden struct {
	// Name is the garden identity used for targeting (YAML key "identity").
	Name string `json:"identity"`
	// Alias is an optional alternative name (YAML key "name").
	Alias string `json:"name,omitempty"`
	// Kubeconfig is the path to the garden cluster kubeconfig.
	Kubeconfig string `json:"kubeconfig"`
	// Context overrides the current-context of the kubeconfig.
	Context string `json:"context,omitempty"`
}

// DisplayName returns the alias if set, otherwise the identity.
func (g Garden) DisplayName() string {
	if g.Alias != "" && g.Alias != g.Name {
		return fmt.Sprintf("%s (%s)", g.Name, g.Alias)
	}
	return g.Name
}

// Gardenctl is the subset of the gardenctl-v2 configuration that gtui needs.
type Gardenctl struct {
	Filename       string   `json:"-"`
	LinkKubeconfig *bool    `json:"linkKubeconfig,omitempty"`
	Gardens        []Garden `json:"gardens"`
}

// HomeDir returns the gardenctl home directory: $GCTL_HOME or ~/.garden.
func HomeDir() (string, error) {
	if h := os.Getenv(envHome); h != "" {
		return ExpandHome(h)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, homeFolder), nil
}

// GardenctlPath returns the path of the gardenctl config file the same way
// gardenctl resolves it: $GCTL_HOME (or ~/.garden) / ($GCTL_CONFIG_NAME or gardenctl-v2).yaml.
func GardenctlPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	name := defaultConfigName
	if n := os.Getenv(envConfigName); n != "" {
		name = n
	}
	return filepath.Join(dir, name+".yaml"), nil
}

// LoadGardenctl reads the gardenctl configuration from filename.
func LoadGardenctl(filename string) (*Gardenctl, error) {
	buf, err := os.ReadFile(filename) // #nosec G304 -- user-provided config path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("gardenctl config %s not found: %w", filename, err)
		}
		return nil, err
	}
	cfg := &Gardenctl{Filename: filename}
	if err := yaml.Unmarshal(buf, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}
	for i, g := range cfg.Gardens {
		if g.Name == "" {
			return nil, fmt.Errorf("parse %s: garden at index %d has no identity", filename, i)
		}
		if cfg.Gardens[i].Kubeconfig, err = ExpandHome(g.Kubeconfig); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// Garden returns the garden with the given identity or alias.
func (c *Gardenctl) Garden(name string) (Garden, bool) {
	for _, g := range c.Gardens {
		if g.Name == name {
			return g, true
		}
	}
	for _, g := range c.Gardens {
		if g.Alias == name {
			return g, true
		}
	}
	return Garden{}, false
}

// ExpandHome replaces a leading "~" with the user's home directory.
func ExpandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}
