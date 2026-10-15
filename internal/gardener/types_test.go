// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"reflect"
	"testing"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShootDetails(t *testing.T) {
	done := metav1.NewTime(time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC))
	s := ShootFromAPI(shoot("garden-a", "s1", "seed-1", func(s *gardencorev1beta1.Shoot) {
		s.Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("s1.a.example.com")}
		s.Status.AdvertisedAddresses = []gardencorev1beta1.ShootAdvertisedAddress{
			{Name: "internal", URL: "https://internal"}, {Name: "external", URL: "https://api.s1.a.example.com"}}
		s.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
			Name: "pool-a", Minimum: 1, Maximum: 3, Zones: []string{"eu-1a", "eu-1b"},
			Machine: gardencorev1beta1.Machine{Type: "m5.large", Image: &gardencorev1beta1.ShootMachineImage{Name: "gardenlinux", Version: ptr.To("1592.1.0")}},
		}}
		s.Spec.Maintenance = &gardencorev1beta1.Maintenance{
			TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{Begin: "220000+0100", End: "230000+0100"},
			AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{KubernetesVersion: true, MachineImageVersion: ptr.To(false)},
		}
		s.Spec.Hibernation = &gardencorev1beta1.Hibernation{Schedules: []gardencorev1beta1.HibernationSchedule{
			{Start: ptr.To("00 20 * * 1-5"), End: ptr.To("00 08 * * 1-5"), Location: ptr.To("Europe/Berlin")}}}
		s.Status.Credentials = &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{
			CertificateAuthorities: &gardencorev1beta1.CARotation{Phase: gardencorev1beta1.RotationPrepared},
			SSHKeypair:             &gardencorev1beta1.ShootSSHKeypairRotation{LastCompletionTime: &done},
		}}
	}))
	if s.Domain != "s1.a.example.com" || s.APIServer != "https://api.s1.a.example.com" {
		t.Errorf("domain %q, api server %q", s.Domain, s.APIServer)
	}
	wantWorkers := []Worker{{Name: "pool-a", MachineType: "m5.large", Image: "gardenlinux 1592.1.0", Min: 1, Max: 3, Zones: []string{"eu-1a", "eu-1b"}}}
	if !reflect.DeepEqual(s.Workers, wantWorkers) {
		t.Errorf("workers %+v", s.Workers)
	}
	if s.MaintenanceWindow != "22:00-23:00 +0100" || !reflect.DeepEqual(s.AutoUpdate, []string{"Kubernetes"}) {
		t.Errorf("maintenance %q %v", s.MaintenanceWindow, s.AutoUpdate)
	}
	if want := []string{"hibernate 00 20 * * 1-5, wake up 00 08 * * 1-5 (Europe/Berlin)"}; !reflect.DeepEqual(s.HibernationSchedules, want) {
		t.Errorf("schedules %q", s.HibernationSchedules)
	}
	wantRot := []Rotation{{Name: "Certificate authorities", Phase: "Prepared"}, {Name: "SSH key pair", LastCompleted: done.Time}}
	if !reflect.DeepEqual(s.Rotations, wantRot) {
		t.Errorf("rotations %+v", s.Rotations)
	}
}
