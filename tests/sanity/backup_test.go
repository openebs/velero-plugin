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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"

	app "github.com/openebs/velero-plugin/tests/app"
	k8s "github.com/openebs/velero-plugin/tests/k8s"
	openebs "github.com/openebs/velero-plugin/tests/openebs"
	velero "github.com/openebs/velero-plugin/tests/velero"
)

const (
	// AppNs application namespace
	AppNs = "test"

	// TargetedNs namespace used for restore in different namespace
	TargetedNs = "ns1"

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

	err = app.CreateNamespace(TargetedNs)
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
			var isExist bool

			By("Creating a backup")

			err = openebs.Client.WaitForHealthyCVR(openebs.AppPVC)
			Expect(err).NotTo(HaveOccurred(), "No healthy CVR for %s", openebs.AppPVC)
			// There are chances that istgt is not updated, but replica is healthy
			time.Sleep(30 * time.Second)

			backupName, status, err = velero.Client.CreateBackup(AppNs)
			if (err != nil) || status != v1.BackupPhaseCompleted {
				_ = velero.Client.DumpBackupLogs(backupName)
				openebs.Client.DumpLogs()
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to create backup=%s for namespace=%s", backupName, AppNs)
			Expect(status).To(Equal(v1.BackupPhaseCompleted), "Backup=%s for namespace=%s failed", backupName, AppNs)

			isExist, err = openebs.Client.IsBackupResourcesExist(backupName, app.PVCName, AppNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to verify snapshot cleanup for backup=%s", backupName)
			Expect(isExist).To(BeFalse(), "Snapshot for backup=%s still exist", backupName)
		})
	})

	Context("Scheduled Backup", func() {
		It("Backup Test 1", func() {
			var status v1.BackupPhase
			var isExist bool

			By("Creating a scheduled backup")
			scheduleName, status, err = velero.Client.CreateSchedule(AppNs, "*/2 * * * *", 3)
			Expect(err).NotTo(HaveOccurred(), "Failed create schedule:%s status=%s", scheduleName, status)
			Expect(status).To(Equal(v1.BackupPhaseCompleted), "Schedule=%s failed", scheduleName)

			err = velero.Client.DeleteSchedule(scheduleName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete schedule=%s", scheduleName)

			bkplist, serr := velero.Client.GetScheduledBackups(scheduleName)
			Expect(serr).NotTo(HaveOccurred(), "Failed to get backup list for schedule=%s", scheduleName)

			for i, bkp := range bkplist {
				isExist, err = openebs.Client.IsBackupResourcesExist(bkp, app.PVCName, AppNs)
				Expect(err).NotTo(HaveOccurred(),
					"Failed to verify snapshot cleanup for backup=%s, with incremental count=%d", bkp, i)
				Expect(isExist).To(BeFalse(), "Snapshot for backup=%s, with incremental count=%d, still exist", bkp, i)
			}
		})
	})

	Context("Restore Test", func() {
		BeforeEach(func() {
			By("Destroying Application and Volume")
			err = app.DestroyApplication(app.BusyboxYaml, AppNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to destroy application in namespace=%s", AppNs)
			err = openebs.Client.DeleteVolume(openebs.PVCYaml, AppNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete volume for namespace=%s", AppNs)
		})

		It("Restore from non-scheduled backup", func() {
			var (
				status v1.RestorePhase
				phase  corev1.PersistentVolumeClaimPhase
			)

			By("Restoring from a non-scheduled backup")
			status, err = velero.Client.CreateRestore(AppNs, AppNs, backupName, "")
			if err != nil || status != v1.RestorePhaseCompleted {
				dumpLogs()
			}

			Expect(err).NotTo(HaveOccurred(), "Failed to create a restore from backup=%s", backupName)
			Expect(status).To(Equal(v1.RestorePhaseCompleted), "Restore from backup=%s failed", backupName)

			By("Checking if restored PVC is bound or not")
			phase, perr := k8s.Client.GetPVCPhase(app.PVCName, AppNs)
			Expect(perr).NotTo(HaveOccurred(), "Failed to verify PVC=%s bound status for namespace=%s", app.PVCName, AppNs)
			Expect(phase).To(Equal(corev1.ClaimBound), "PVC=%s not bound", app.PVCName)

			By("Checking if restored CVR are in healthy state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, AppNs, v1alpha1.CVRStatusOnline)
			Expect(ok).To(BeTrue(), "CVR for PVC=%s are not in errored state", app.PVCName)
		})

		It("Restore from scheduled backup", func() {
			var (
				status v1.RestorePhase
				phase  corev1.PersistentVolumeClaimPhase
			)

			By("Restoring from a scheduled backup")
			status, err = velero.Client.CreateRestore(AppNs, AppNs, "", scheduleName)
			if err != nil || status != v1.RestorePhaseCompleted {
				dumpLogs()
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to create a restore from schedule=%s", scheduleName)
			Expect(status).To(Equal(v1.RestorePhaseCompleted), "Restore from schedule=%s failed", scheduleName)

			By("Checking if restored PVC is bound or not")
			phase, err = k8s.Client.GetPVCPhase(app.PVCName, AppNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to verify PVC=%s bound status for namespace=%s", app.PVCName, AppNs)
			Expect(phase).To(Equal(corev1.ClaimBound), "PVC=%s not bound", app.PVCName)

			By("Checking if restored CVR are in healthy state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, AppNs, v1alpha1.CVRStatusOnline)
			Expect(ok).To(BeTrue(), "CVR for PVC=%s are not in errored state", app.PVCName)

			By("Checking if restore has created snapshot or not")
			snapshotList, serr := velero.Client.GetRestoredSnapshotFromSchedule(scheduleName)
			Expect(serr).NotTo(HaveOccurred())
			for snapshot := range snapshotList {
				ok, err = openebs.Client.CheckSnapshot(app.PVCName, AppNs, snapshot)
				if err != nil {
					dumpLogs()
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to verify restored snapshot from schedule=%s", scheduleName)
				Expect(ok).Should(BeTrue(), "Snapshots are not restored from schedule=%s", scheduleName)
			}
		})
	})

	Context("Restore Test in different namespace", func() {
		AfterEach(func() {
			By("Destroying Application and Volume")
			err = app.DestroyApplication(app.BusyboxYaml, TargetedNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to destroy application in namespace=%s", TargetedNs)
			err = openebs.Client.DeleteVolume(openebs.PVCYaml, TargetedNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete volume for namespace=%s", TargetedNs)
		})

		It("Restore from non-scheduled backup to different Namespace", func() {
			var status v1.RestorePhase

			By("Restoring from a non-scheduled backup to a different namespace")
			status, err = velero.Client.CreateRestore(AppNs, TargetedNs, backupName, "")
			if err != nil || status != v1.RestorePhaseCompleted {
				dumpLogs()
			}

			Expect(err).NotTo(HaveOccurred(), "Failed to create a restore from backup=%s", backupName)
			Expect(status).To(Equal(v1.RestorePhaseCompleted), "Restore from backup=%s failed", backupName)

			By("Checking if restored PVC is bound or not")
			phase, perr := k8s.Client.GetPVCPhase(app.PVCName, TargetedNs)
			Expect(perr).NotTo(HaveOccurred(), "Failed to verify PVC=%s bound status for namespace=%s", app.PVCName, TargetedNs)
			Expect(phase).To(Equal(corev1.ClaimBound), "PVC=%s not bound", app.PVCName)

			By("Checking if restored CVR are in healthy state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, TargetedNs, v1alpha1.CVRStatusOnline)
			Expect(ok).To(BeTrue(), "CVR for PVC=%s is not in errored state", app.PVCName)
		})

		It("Restore from scheduled backup to different Namespace", func() {
			var status v1.RestorePhase

			By("Restoring from a scheduled backup to a different namespace")
			status, err = velero.Client.CreateRestore(AppNs, TargetedNs, "", scheduleName)
			if err != nil || status != v1.RestorePhaseCompleted {
				dumpLogs()
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to create a restore from schedule=%s", scheduleName)
			Expect(status).To(Equal(v1.RestorePhaseCompleted), "Restore from schedule=%s failed", scheduleName)

			By("Checking if restored PVC is bound or not")
			phase, err := k8s.Client.GetPVCPhase(app.PVCName, TargetedNs)
			Expect(err).NotTo(HaveOccurred(), "Failed to verify PVC=%s bound status for namespace=%s", app.PVCName, TargetedNs)
			Expect(phase).To(Equal(corev1.ClaimBound), "PVC=%s not bound", app.PVCName)

			By("Checking if restored CVR are in healthy state")
			ok := openebs.Client.CheckCVRStatus(app.PVCName, TargetedNs, v1alpha1.CVRStatusOnline)
			Expect(ok).To(BeTrue(), "CVR for PVC=%s is not in errored state", app.PVCName)

			By("Checking if restore has created snapshot or not")
			snapshotList, err := velero.Client.GetRestoredSnapshotFromSchedule(scheduleName)
			Expect(err).NotTo(HaveOccurred())
			for snapshot := range snapshotList {
				ok, err := openebs.Client.CheckSnapshot(app.PVCName, TargetedNs, snapshot)
				if err != nil {
					dumpLogs()
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to verify restored snapshot from schedule=%s", scheduleName)
				Expect(ok).Should(BeTrue(), "Snapshots are not restored from schedule=%s", scheduleName)
			}
		})
	})
})

func dumpLogs() {
	velero.Client.DumpLogs()
	openebs.Client.DumpLogs()
}
