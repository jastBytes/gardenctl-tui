// Package gardener reads Projects, Shoots and Seeds from a garden cluster.
package gardener

import (
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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
