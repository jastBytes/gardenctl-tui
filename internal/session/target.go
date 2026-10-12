// Package session prepares isolated gardenctl sessions for launched tools.
package session

import "strings"

// Target identifies a cluster to launch a tool against.
type Target struct {
	Garden string
	// Project is set when the shoot was reached through a project.
	Project string
	// Seed is set when the shoot was reached through a seed (or for seed targets).
	Seed  string
	Shoot string
	// Namespace is the shoot's project namespace (for the {namespace} placeholder).
	Namespace string
	// ControlPlane targets the shoot's control plane on its seed.
	ControlPlane bool
}

// String returns garden/project/shoot (or garden/seed/shoot), as used in GTUI_TARGET.
func (t Target) String() string {
	parts := []string{t.Garden}
	if t.Project != "" {
		parts = append(parts, t.Project)
	} else if t.Seed != "" {
		parts = append(parts, t.Seed)
	}
	if t.Shoot != "" {
		parts = append(parts, t.Shoot)
	}
	s := strings.Join(parts, "/")
	if t.ControlPlane {
		s += " (control plane)"
	}
	return s
}

// TargetArgs returns the `gardenctl target` arguments for t.
func (t Target) TargetArgs() []string {
	args := []string{"target", "--garden", t.Garden}
	switch {
	case t.Project != "":
		args = append(args, "--project", t.Project)
	case t.Seed != "":
		args = append(args, "--seed", t.Seed)
	}
	if t.Shoot != "" {
		args = append(args, "--shoot", t.Shoot)
		if t.ControlPlane {
			args = append(args, "--control-plane")
		}
	}
	return args
}
