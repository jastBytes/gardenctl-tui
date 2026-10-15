// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Package gardener reads Projects, Shoots and Seeds from a garden cluster.
package gardener

import (
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status is the aggregated health of a shoot or seed.
type Status int

const (
	StatusUnknown Status = iota
	StatusHealthy
	StatusProgressing
	StatusError
	StatusHibernated
)

func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "Healthy"
	case StatusProgressing:
		return "Progressing"
	case StatusError:
		return "Error"
	case StatusHibernated:
		return "Hibernated"
	default:
		return "Unknown"
	}
}

// Project is a Gardener project.
type Project struct {
	Name      string
	Namespace string
	Phase     string
	Purpose   string
	Owner     string
}

// Seed is a Gardener seed.
type Seed struct {
	Name     string
	Provider string
	Region   string
	Status   Status
}

// Condition is a simplified shoot condition.
type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
	Since   time.Time
}

// Shoot is the view model of a Gardener shoot.
type Shoot struct {
	Name        string
	Namespace   string
	Project     string
	Seed        string
	Version     string
	Provider    string
	Region      string
	Purpose     string
	TechnicalID string
	Hibernated  bool
	Deleting    bool
	Status      Status
	LastOp      LastOperation
	Conditions  []Condition
	Constraints []Condition
	LastErrors  []string
	Created     time.Time

	// Domain is the shoot's DNS domain; APIServer the external API server URL.
	Domain    string
	APIServer string
	Workers   []Worker
	// MaintenanceWindow is e.g. "22:00-23:00 +0000" ("" if unset).
	MaintenanceWindow string
	// AutoUpdate lists what maintenance updates automatically ("Kubernetes", "machine images").
	AutoUpdate []string
	// HibernationSchedules are e.g. "00 20 * * 1-5 → 00 08 * * 1-5 (Europe/Berlin)".
	HibernationSchedules []string
	Rotations            []Rotation
}

// Worker summarises a worker pool.
type Worker struct {
	Name        string
	MachineType string
	// Image is "name version".
	Image    string
	Min, Max int32
	Zones    []string
}

// Rotation is the state of one credentials rotation.
type Rotation struct {
	Name string
	// Phase is the rotation phase, e.g. "Prepared" ("" for kinds without phases).
	Phase         string
	LastCompleted time.Time
}

// LastOperation summarises shoot.status.lastOperation.
type LastOperation struct {
	Type     string
	State    string
	Progress int32
	Message  string
	Updated  time.Time
}

func (o LastOperation) String() string {
	if o.Type == "" {
		return "-"
	}
	if o.State == string(gardencorev1beta1.LastOperationStateSucceeded) {
		return fmt.Sprintf("%s %s", o.Type, o.State)
	}
	return fmt.Sprintf("%s %s (%d%%)", o.Type, o.State, o.Progress)
}

// ProjectFromAPI converts an API project.
func ProjectFromAPI(p *gardencorev1beta1.Project) Project {
	out := Project{Name: p.Name, Phase: string(p.Status.Phase)}
	if p.Spec.Namespace != nil {
		out.Namespace = *p.Spec.Namespace
	}
	if p.Spec.Purpose != nil {
		out.Purpose = *p.Spec.Purpose
	}
	if p.Spec.Owner != nil {
		out.Owner = p.Spec.Owner.Name
	}
	return out
}

// SeedFromAPI converts an API seed.
func SeedFromAPI(s *gardencorev1beta1.Seed) Seed {
	out := Seed{Name: s.Name, Provider: s.Spec.Provider.Type, Region: s.Spec.Provider.Region, Status: StatusHealthy}
	if len(s.Status.Conditions) == 0 {
		out.Status = StatusUnknown
	}
	out.Status = worst(out.Status, conditionsStatus(s.Status.Conditions))
	if s.Status.LastOperation != nil {
		out.Status = worst(out.Status, lastOpStatus(s.Status.LastOperation.State))
	}
	return out
}

// ProjectFromNamespace derives the project name from a project namespace ("garden-foo" → "foo").
func ProjectFromNamespace(ns string) string {
	if ns == "garden" {
		return "garden"
	}
	return strings.TrimPrefix(ns, "garden-")
}

// ShootFromAPI converts an API shoot and computes its status.
func ShootFromAPI(s *gardencorev1beta1.Shoot) Shoot {
	out := Shoot{
		Name:        s.Name,
		Namespace:   s.Namespace,
		Project:     ProjectFromNamespace(s.Namespace),
		Version:     s.Spec.Kubernetes.Version,
		Provider:    s.Spec.Provider.Type,
		Region:      s.Spec.Region,
		TechnicalID: s.Status.TechnicalID,
		Hibernated:  s.Status.IsHibernated,
		Deleting:    s.DeletionTimestamp != nil,
		Created:     s.CreationTimestamp.Time,
	}
	if s.Spec.SeedName != nil {
		out.Seed = *s.Spec.SeedName
	}
	if s.Spec.Purpose != nil {
		out.Purpose = string(*s.Spec.Purpose)
	}
	if lo := s.Status.LastOperation; lo != nil {
		out.LastOp = LastOperation{
			Type:     string(lo.Type),
			State:    string(lo.State),
			Progress: lo.Progress,
			Message:  lo.Description,
			Updated:  lo.LastUpdateTime.Time,
		}
	}
	if s.Spec.DNS != nil && s.Spec.DNS.Domain != nil {
		out.Domain = *s.Spec.DNS.Domain
	}
	for _, a := range s.Status.AdvertisedAddresses {
		if a.Name == "external" {
			out.APIServer = a.URL
		}
	}
	for _, w := range s.Spec.Provider.Workers {
		worker := Worker{Name: w.Name, MachineType: w.Machine.Type, Min: w.Minimum, Max: w.Maximum, Zones: w.Zones}
		if img := w.Machine.Image; img != nil {
			worker.Image = img.Name
			if img.Version != nil {
				worker.Image += " " + *img.Version
			}
		}
		out.Workers = append(out.Workers, worker)
	}
	if m := s.Spec.Maintenance; m != nil {
		if tw := m.TimeWindow; tw != nil {
			out.MaintenanceWindow = formatWindow(tw.Begin, tw.End)
		}
		if au := m.AutoUpdate; au != nil {
			if au.KubernetesVersion {
				out.AutoUpdate = append(out.AutoUpdate, "Kubernetes")
			}
			if au.MachineImageVersion == nil || *au.MachineImageVersion {
				out.AutoUpdate = append(out.AutoUpdate, "machine images")
			}
		}
	}
	if h := s.Spec.Hibernation; h != nil {
		for _, sc := range h.Schedules {
			out.HibernationSchedules = append(out.HibernationSchedules, formatSchedule(sc))
		}
	}
	out.Rotations = rotations(s.Status.Credentials)
	out.Conditions = convertConditions(s.Status.Conditions)
	out.Constraints = convertConditions(s.Status.Constraints)
	for _, e := range s.Status.LastErrors {
		out.LastErrors = append(out.LastErrors, e.Description)
	}
	out.Status = shootStatus(s)
	return out
}

func convertConditions(in []gardencorev1beta1.Condition) []Condition {
	out := make([]Condition, 0, len(in))
	for _, c := range in {
		out = append(out, Condition{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
			Since:   c.LastTransitionTime.Time,
		})
	}
	return out
}

// shootStatus aggregates a shoot's health:
// hibernated → grey, failed op / errors / False condition → red,
// running op / Progressing or Unknown condition → yellow, otherwise green.
func shootStatus(s *gardencorev1beta1.Shoot) Status {
	hibernationWanted := s.Spec.Hibernation != nil && s.Spec.Hibernation.Enabled != nil && *s.Spec.Hibernation.Enabled
	lo := s.Status.LastOperation
	opDone := lo == nil || lo.State == gardencorev1beta1.LastOperationStateSucceeded

	if s.Status.IsHibernated && hibernationWanted && opDone && s.DeletionTimestamp == nil {
		return StatusHibernated
	}
	if lo == nil && len(s.Status.Conditions) == 0 {
		return StatusUnknown
	}
	st := StatusHealthy
	if len(s.Status.LastErrors) > 0 {
		st = StatusError
	}
	if lo != nil {
		st = worst(st, lastOpStatus(lo.State))
	}
	if s.DeletionTimestamp != nil {
		st = worst(st, StatusProgressing)
	}
	return worst(st, conditionsStatus(s.Status.Conditions))
}

func lastOpStatus(state gardencorev1beta1.LastOperationState) Status {
	switch state {
	case gardencorev1beta1.LastOperationStateSucceeded:
		return StatusHealthy
	case gardencorev1beta1.LastOperationStateError,
		gardencorev1beta1.LastOperationStateFailed,
		gardencorev1beta1.LastOperationStateAborted:
		return StatusError
	default:
		return StatusProgressing
	}
}

func conditionsStatus(conds []gardencorev1beta1.Condition) Status {
	st := StatusHealthy
	for _, c := range conds {
		switch c.Status {
		case gardencorev1beta1.ConditionTrue:
		case gardencorev1beta1.ConditionFalse:
			st = worst(st, StatusError)
		default:
			st = worst(st, StatusProgressing)
		}
	}
	return st
}

// worst returns the more severe of two statuses (Error > Progressing > Unknown > Healthy).
func worst(a, b Status) Status {
	rank := func(s Status) int {
		switch s {
		case StatusError:
			return 4
		case StatusProgressing:
			return 3
		case StatusUnknown:
			return 2
		case StatusHealthy:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// formatWindow turns "220000+0100" and "230000+0100" into "22:00-23:00 +0100".
func formatWindow(begin, end string) string {
	hm := func(t string) string {
		if len(t) < 4 {
			return t
		}
		return t[0:2] + ":" + t[2:4]
	}
	zone := ""
	if len(begin) > 6 {
		zone = " " + begin[6:]
	}
	return hm(begin) + "-" + hm(end) + zone
}

func formatSchedule(sc gardencorev1beta1.HibernationSchedule) string {
	deref := func(p *string) string {
		if p == nil {
			return "-"
		}
		return *p
	}
	out := "hibernate " + deref(sc.Start) + ", wake up " + deref(sc.End)
	if sc.Location != nil {
		out += " (" + *sc.Location + ")"
	}
	return out
}

func rotations(c *gardencorev1beta1.ShootCredentials) []Rotation {
	if c == nil || c.Rotation == nil {
		return nil
	}
	r := c.Rotation
	var out []Rotation
	add := func(name string, phase gardencorev1beta1.CredentialsRotationPhase, done *metav1.Time) {
		rot := Rotation{Name: name, Phase: string(phase)}
		if done != nil {
			rot.LastCompleted = done.Time
		}
		out = append(out, rot)
	}
	if ca := r.CertificateAuthorities; ca != nil {
		add("Certificate authorities", ca.Phase, ca.LastCompletionTime)
	}
	if sa := r.ServiceAccountKey; sa != nil {
		add("Service account key", sa.Phase, sa.LastCompletionTime)
	}
	if e := r.ETCDEncryptionKey; e != nil {
		add("ETCD encryption key", e.Phase, e.LastCompletionTime)
	}
	if k := r.SSHKeypair; k != nil {
		add("SSH key pair", "", k.LastCompletionTime)
	}
	if o := r.Observability; o != nil {
		add("Observability", "", o.LastCompletionTime)
	}
	return out
}
