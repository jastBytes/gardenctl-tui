// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func shoot(ns, name, seed string, mutate ...func(*gardencorev1beta1.Shoot)) *gardencorev1beta1.Shoot {
	s := &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: gardencorev1beta1.ShootSpec{
			SeedName:   ptr.To(seed),
			Region:     "eu-1",
			Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.33.1"},
			Provider:   gardencorev1beta1.Provider{Type: "aws"},
		},
		Status: gardencorev1beta1.ShootStatus{
			TechnicalID: "shoot--" + ProjectFromNamespace(ns) + "--" + name,
			LastOperation: &gardencorev1beta1.LastOperation{
				Type: gardencorev1beta1.LastOperationTypeReconcile, State: gardencorev1beta1.LastOperationStateSucceeded, Progress: 100,
			},
			Conditions: []gardencorev1beta1.Condition{{Type: "APIServerAvailable", Status: gardencorev1beta1.ConditionTrue}},
		},
	}
	for _, m := range mutate {
		m(s)
	}
	return s
}

func newFake(objs ...client.Object) Client {
	c := fake.NewClientBuilder().
		WithScheme(Scheme()).
		WithObjects(objs...).
		WithIndex(&gardencorev1beta1.Shoot{}, SeedNameField, func(o client.Object) []string {
			if s := o.(*gardencorev1beta1.Shoot).Spec.SeedName; s != nil {
				return []string{*s}
			}
			return nil
		}).
		Build()
	return New(c)
}

func TestClient(t *testing.T) {
	ctx := context.Background()
	c := newFake(
		&gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "b-proj"},
			Spec:       gardencorev1beta1.ProjectSpec{Namespace: ptr.To("garden-b-proj")},
		},
		&gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "a-proj"},
			Spec:       gardencorev1beta1.ProjectSpec{Namespace: ptr.To("garden-a-proj"), Purpose: ptr.To("dev")},
			Status:     gardencorev1beta1.ProjectStatus{Phase: gardencorev1beta1.ProjectReady},
		},
		&gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: "seed-1"},
			Spec:       gardencorev1beta1.SeedSpec{Provider: gardencorev1beta1.SeedProvider{Type: "aws", Region: "eu-1"}},
		},
		shoot("garden-a-proj", "s2", "seed-1"),
		shoot("garden-a-proj", "s1", "seed-2"),
		shoot("garden-b-proj", "s3", "seed-1"),
	)

	projects, err := c.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].Name != "a-proj" || projects[0].Namespace != "garden-a-proj" ||
		projects[0].Purpose != "dev" || projects[0].Phase != "Ready" {
		t.Fatalf("projects: %+v", projects)
	}

	shoots, err := c.ListShoots(ctx, "garden-a-proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(shoots) != 2 || shoots[0].Name != "s1" || shoots[1].Name != "s2" || shoots[0].Project != "a-proj" {
		t.Fatalf("shoots: %+v", shoots)
	}

	all, err := c.ListShoots(ctx, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("all shoots: %v %+v", err, all)
	}

	onSeed, err := c.ListShootsOnSeed(ctx, "seed-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(onSeed) != 2 || onSeed[0].Name != "s2" || onSeed[1].Name != "s3" {
		t.Fatalf("on seed: %+v", onSeed)
	}

	seeds, err := c.ListSeeds(ctx)
	if err != nil || len(seeds) != 1 || seeds[0].Region != "eu-1" || seeds[0].Status != StatusUnknown {
		t.Fatalf("seeds: %v %+v", err, seeds)
	}

	got, err := c.GetShoot(ctx, "garden-a-proj", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Seed != "seed-2" || got.Version != "1.33.1" || got.Status != StatusHealthy || got.TechnicalID != "shoot--a-proj--s1" {
		t.Fatalf("shoot: %+v", got)
	}
	if _, err := c.GetShoot(ctx, "garden-a-proj", "missing"); err == nil {
		t.Fatal("expected not found")
	}

	manifest, err := c.GetShootYAML(ctx, "garden-a-proj", "s1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"apiVersion: core.gardener.cloud/v1beta1", "kind: Shoot", "name: s1", "technicalID: shoot--a-proj--s1"} {
		if !strings.Contains(string(manifest), want) {
			t.Fatalf("yaml lacks %q:\n%s", want, manifest)
		}
	}
	if strings.Contains(string(manifest), "managedFields") {
		t.Fatalf("yaml contains managed fields:\n%s", manifest)
	}
	if _, err := c.GetShootYAML(ctx, "garden-a-proj", "missing"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestShootStatus(t *testing.T) {
	now := metav1.Now()
	cases := map[string]struct {
		mutate func(*gardencorev1beta1.Shoot)
		want   Status
	}{
		"healthy": {func(*gardencorev1beta1.Shoot) {}, StatusHealthy},
		"hibernated": {func(s *gardencorev1beta1.Shoot) {
			s.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: ptr.To(true)}
			s.Status.IsHibernated = true
			s.Status.Conditions[0].Status = gardencorev1beta1.ConditionFalse
		}, StatusHibernated},
		"waking up": {func(s *gardencorev1beta1.Shoot) {
			s.Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: ptr.To(false)}
			s.Status.IsHibernated = true
			s.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
		}, StatusProgressing},
		"reconciling": {func(s *gardencorev1beta1.Shoot) {
			s.Status.LastOperation.State = gardencorev1beta1.LastOperationStateProcessing
		}, StatusProgressing},
		"condition progressing": {func(s *gardencorev1beta1.Shoot) {
			s.Status.Conditions[0].Status = gardencorev1beta1.ConditionProgressing
		}, StatusProgressing},
		"condition false": {func(s *gardencorev1beta1.Shoot) {
			s.Status.Conditions[0].Status = gardencorev1beta1.ConditionFalse
		}, StatusError},
		"op failed": {func(s *gardencorev1beta1.Shoot) {
			s.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
		}, StatusError},
		"last errors": {func(s *gardencorev1beta1.Shoot) {
			s.Status.LastErrors = []gardencorev1beta1.LastError{{Description: "boom"}}
		}, StatusError},
		"deleting": {func(s *gardencorev1beta1.Shoot) {
			s.DeletionTimestamp = &now
		}, StatusProgressing},
		"new": {func(s *gardencorev1beta1.Shoot) {
			s.Status = gardencorev1beta1.ShootStatus{}
		}, StatusUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ShootFromAPI(shoot("garden-p", "s", "seed", tc.mutate))
			if got.Status != tc.want {
				t.Fatalf("got %s, want %s", got.Status, tc.want)
			}
		})
	}
}

func TestLastOperationString(t *testing.T) {
	if (LastOperation{}).String() != "-" {
		t.Fatal("empty")
	}
	if s := (LastOperation{Type: "Reconcile", State: "Succeeded", Progress: 100}).String(); s != "Reconcile Succeeded" {
		t.Fatal(s)
	}
	if s := (LastOperation{Type: "Create", State: "Processing", Progress: 42}).String(); s != "Create Processing (42%)" {
		t.Fatal(s)
	}
}

func TestProjectFromNamespace(t *testing.T) {
	for in, want := range map[string]string{"garden-foo": "foo", "garden": "garden", "other": "other"} {
		if got := ProjectFromNamespace(in); got != want {
			t.Errorf("%s: got %s", in, got)
		}
	}
}

func TestListShootsInChunks(t *testing.T) {
	defer func(n int64) { listChunkSize = n }(listChunkSize)
	listChunkSize = 2
	var all []gardencorev1beta1.Shoot
	for _, n := range []string{"s1", "s2", "s3", "s4", "s5"} {
		all = append(all, *shoot("garden-a", n, "seed-1"))
	}
	for _, tc := range []struct {
		name   string
		expire bool
		calls  []string // continue:limit of each list call
	}{
		{"pages", false, []string{"0:2", "2:2", "4:2"}},
		{"expired continue token lists in one piece", true, []string{"0:2", "2:2", "0:0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			raw := fake.NewClientBuilder().WithScheme(Scheme()).WithInterceptorFuncs(interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, l client.ObjectList, opts ...client.ListOption) error {
					o := (&client.ListOptions{}).ApplyOptions(opts)
					start, _ := strconv.Atoi(o.Continue)
					calls = append(calls, fmt.Sprintf("%d:%d", start, o.Limit))
					if tc.expire && o.Continue == "2" {
						return apierrors.NewResourceExpired("continue token expired")
					}
					list := l.(*gardencorev1beta1.ShootList)
					end := len(all)
					if o.Limit > 0 && start+int(o.Limit) < end {
						end = start + int(o.Limit)
						list.Continue = strconv.Itoa(end)
					}
					list.Items = all[start:end]
					return nil
				},
			}).Build()
			shoots, err := New(raw).ListShoots(context.Background(), "garden-a")
			if err != nil {
				t.Fatal(err)
			}
			if got := shootNames(shoots); strings.Join(got, ",") != "s1,s2,s3,s4,s5" {
				t.Errorf("shoots = %v", got)
			}
			if strings.Join(calls, " ") != strings.Join(tc.calls, " ") {
				t.Errorf("calls = %v, want %v", calls, tc.calls)
			}
		})
	}
}
