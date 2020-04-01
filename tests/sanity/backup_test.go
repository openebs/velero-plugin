/*
Copyright 2019 The OpenEBS Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sanity

import (
	"testing"
	"time"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	app "github.com/openebs/velero-plugin/tests/app"
	k8s "github.com/openebs/velero-plugin/tests/k8s"
	openebs "github.com/openebs/velero-plugin/tests/openebs"
	velero "github.com/openebs/velero-plugin/tests/velero"
	corev1 "k8s.io/api/core/v1"
)

const (
	AppNs            = "test"
	BackupLocation   = "default"
	SnapshotLocation = "default"
)

func TestVELERO(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Velero integration test suite")
}

var (
	err          error
	backupName   string
	scheduleName string
)

var _ = BeforeSuite(func() {
	err = app.CreateNamespace(AppNs)
	Expect(err).NotTo(HaveOccurred())

	err = k8s.Client.CreateStorageClass(openebs.SCYaml)
	Expect(err).NotTo(HaveOccurred())

	err = openebs.Client.CreateSPC(openebs.SPCYaml)
	Expect(err).NotTo(HaveOccurred())

	err = openebs.Client.CreateVolume(openebs.PVCYaml, AppNs, true)
	Expect(err).NotTo(HaveOccurred())

	err = app.DeployApplication(app.BusyboxYaml, AppNs)
	Expect(err).NotTo(HaveOccurred())

	velero.BackupLocation = BackupLocation
	velero.SnapshotLocation = SnapshotLocation
})

var _ = Describe("Backup/Restore Test", func() {
	Context("Non-scheduled Backup", func() {
		It("Backup Test 1", func() {
			var status v1.BackupPhase
			By("Creating a backup")

			err = openebs.Client.WaitForHealthyCVR(openebs.AppPVC)
			Expect(err).NotTo(HaveOccurred())
			// There are chances that istgt is not updated, but replica is healthy
			time.Sleep(30 * time.Second)

			backupName, status, err = velero.Client.CreateBackup(AppNs)
			if ((err != nil) || status != v1.BackupPhaseCompleted) &&
				len(backupName) != 0 {
				_ = velero.Client.DumpBackupLogs(backupName)
				_ = openebs.Client.DumpLogs()
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(v1.BackupPhaseCompleted))
			isExist, err := openebs.Client.IsBackupResourcesExist(backupName, app.PVCName, AppNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(isExist).To(BeFalse())
		})
	})

	Context("Scheduled Backup", func() {
		It("Backup Test 1", func() {
			var status v1.BackupPhase

			By("Creating a scheduled backup")
			scheduleName, status, err = velero.Client.CreateSchedule(AppNs, "*/2 * * * *", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(v1.BackupPhaseCompleted))
			err = velero.Client.DeleteSchedule(scheduleName)
			Expect(err).NotTo(HaveOccurred())

			bkplist, err := velero.Client.GetScheduledBackups(scheduleName)
			Expect(err).NotTo(HaveOccurred())

			for _, bkp := range bkplist {
				isExist, err := openebs.Client.IsBackupResourcesExist(bkp, app.PVCName, AppNs)
				Expect(err).NotTo(HaveOccurred())
				Expect(isExist).To(BeFalse())
			}
		})
	})

	Context("Restore Test", func() {
		BeforeEach(func() {
			By("Destroying Application and Volume")
			err = app.DestroyApplication(app.BusyboxYaml, AppNs)
			Expect(err).NotTo(HaveOccurred())
			err = openebs.Client.DeleteVolume(openebs.PVCYaml, AppNs)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Restore from non-scheduled backup Test 1", func() {
			var status v1.RestorePhase

			By("Restoring from a non-scheduled backup")
			status, err = velero.Client.CreateRestore(AppNs, backupName)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(v1.RestorePhaseCompleted))

			By("Checking if restored PVC is bound or not")
			phase, err := k8s.Client.GetPVCPhase(app.PVCName, AppNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(phase).To(Equal(corev1.ClaimBound))

			By("Checking if restored CVR are in error state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, AppNs, v1alpha1.CVRStatusError)
			Expect(ok).To(BeTrue())
		})

		It("Restore from scheduled backup Test 1", func() {
			var status v1.RestorePhase

			By("Restoring from a scheduled backup")
			status, err = velero.Client.CreateRestoreFromSchedule(AppNs, scheduleName, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(v1.RestorePhasePartiallyFailed))

		})

		It("Restore from scheduled backup Test 2", func() {
			var status v1.RestorePhase

			By("Restoring from a scheduled backup")
			status, err = velero.Client.CreateRestoreFromSchedule(AppNs, scheduleName, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal(v1.RestorePhaseCompleted))

			By("Checking if restored PVC is bound or not")
			phase, err := k8s.Client.GetPVCPhase(app.PVCName, AppNs)
			Expect(err).NotTo(HaveOccurred())
			Expect(phase).To(Equal(corev1.ClaimBound))

			By("Checking if restored CVR are in error state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, AppNs, v1alpha1.CVRStatusError)
			Expect(ok).To(BeTrue())

			By("Checking if restore has created Snapshot or not")
			snapshotList, err := velero.Client.GetRestoredSnapshotFromSchedule(scheduleName)
			Expect(err).NotTo(HaveOccurred())
			for snapshot := range snapshotList {
				ok, err := openebs.Client.CheckSnapshot(app.PVCName, AppNs, snapshot)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).Should(BeTrue())
			}
		})
	})
})
