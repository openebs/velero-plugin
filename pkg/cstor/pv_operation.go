/*
Copyright 2020 The OpenEBS Authors.

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

package cstor

import (
	"encoding/json"
	"sort"
	"strings"

	uuid "github.com/gofrs/uuid"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openebs/velero-plugin/pkg/clouduploader"
)

const (
	// PvClonePrefix  prefix for clone volume in case restore from local backup
	PvClonePrefix = "cstor-clone-"
)

func (p *Plugin) updateVolCASInfo(data []byte, volumeID string) error {
	var cas v1alpha1.CASVolume

	err := json.Unmarshal(data, &cas)
	if err != nil {
		return err
	}

	vol := p.volumes[volumeID]
	if vol == nil {
		return errors.Errorf("Volume{%s} not found in volume list", volumeID)
	}
	vol.iscsi = v1.ISCSIPersistentVolumeSource{
		TargetPortal: cas.Spec.TargetPortal,
		IQN:          cas.Spec.Iqn,
		Lun:          cas.Spec.Lun,
		FSType:       cas.Spec.FSType,
		ReadOnly:     false,
	}
	return nil
}

// restoreVolumeFromCloud restore remote snapshot for the given volume
// Note: cstor snapshots are incremental in nature, so restore will be executed
// from base snapshot to incremental snapshot 'vol.backupName'.
func (p *Plugin) restoreVolumeFromCloud(vol *Volume) error {
	// Since we are supporting incremental backups for volume
	// We need to restore all the snapshots from base backup to the target backup
	targetBackupName := vol.backupName

	snapshotList, err := p.getSnapListFromCloud(vol)
	if err != nil {
		return err
	}

	// snapshots are created using timestamp, we need to sort it in ascending order
	sort.Strings(snapshotList)

	for _, snap := range snapshotList {
		p.Log.Infof("Restoring snapshot=%s", snap)

		vol.backupName = snap

		err := p.restoreSnapshotFromCloud(vol)
		if err != nil {
			return errors.Wrapf(err, "failed to restor snapshot=%s", snap)
		}
		p.Log.Infof("Restore of snapshot=%s completed", snap)

		if snap == targetBackupName {
			// we restored till the targetBackupName, no need to restore next snapshot
			break
		}
	}
	return nil
}

// restoreSnapshotFromCloud restore snapshot 'vol.backupName` to volume 'vol.volname'
func (p *Plugin) restoreSnapshotFromCloud(vol *Volume) error {
	p.cl.ExitServer = false

	restore, err := p.sendRestoreRequest(vol)
	if err != nil {
		return errors.Wrapf(err, "Restore request to apiServer failed")
	}

	filename := p.cl.GenerateRemoteFilename(vol.snapshotTag, vol.backupName)
	if filename == "" {
		return errors.Errorf("Error creating remote file name for restore")
	}

	go p.checkRestoreStatus(restore, vol)

	ret := p.cl.Download(filename)
	if !ret {
		return errors.New("failed to restore snapshot")
	}

	if vol.restoreStatus != v1alpha1.RSTCStorStatusDone {
		return errors.Errorf("failed to restore.. status {%s}", vol.restoreStatus)
	}

	return nil
}

func (p *Plugin) getPV(volumeID string) (*v1.PersistentVolume, error) {
	return p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(volumeID, metav1.GetOptions{})
}

func (p *Plugin) restoreVolumeFromLocal(vol *Volume) error {
	_, err := p.sendRestoreRequest(vol)
	if err != nil {
		return errors.Wrapf(err, "Restore request to apiServer failed")
	}
	vol.restoreStatus = v1alpha1.RSTCStorStatusDone
	return nil
}

// getVolumeForLocalRestore return volume information to restore locally for the given volumeID and snapName
// volumeID : pv name from backup
// snapName : snapshot name from where new volume will be created
func (p *Plugin) getVolumeForLocalRestore(volumeID, snapName string) (*Volume, error) {
	pv, err := p.getPV(volumeID)
	if err != nil {
		return nil, errors.Wrapf(err, "error fetching PV=%s", volumeID)
	}

	clonePvName, err := generateClonePVName()
	if err != nil {
		return nil, err
	}
	p.Log.Infof("Renaming PV %s to %s", pv.Name, clonePvName)

	isCSIVolume := isCSIPv(*pv)

	vol := &Volume{
		volname:      clonePvName,
		srcVolname:   pv.Name,
		backupName:   snapName,
		storageClass: pv.Spec.StorageClassName,
		size:         pv.Spec.Capacity[v1.ResourceStorage],
		isCSIVolume:  isCSIVolume,
	}
	p.volumes[vol.volname] = vol
	return vol, nil
}

// getVolumeForRemoteRestore return volume information to restore from remote backup for the given volumeID and snapName
// volumeID : pv name from backup
// snapName : snapshot name from where new volume will be created
func (p *Plugin) getVolumeForRemoteRestore(volumeID, snapName string) (*Volume, error) {
	vol, err := p.createPVC(volumeID, snapName)
	if err != nil {
		p.Log.Errorf("CreatePVC returned error=%s", err)
		return nil, err
	}

	p.Log.Infof("Generated PV name is %s", vol.volname)

	return vol, nil
}

// generateClonePVName return new name for clone pv for the given pv
func generateClonePVName() (string, error) {
	nuuid, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrapf(err, "Error generating uuid for PV rename")
	}

	return PvClonePrefix + nuuid.String(), nil
}

// isCSIPv returns true if given PV is created by cstor CSI driver
func isCSIPv(pv v1.PersistentVolume) bool {
	if pv.Spec.CSI != nil &&
		pv.Spec.CSI.Driver == openebsCSIName {
		return true
	}
	return false
}

// getSnapListFromCloud returns list of all the snapshots(base and incremental) associated with given volume from cloud
func (p *Plugin) getSnapListFromCloud(vol *Volume) ([]string, error) {
	var snapList []string

	scheduleName := p.getScheduleName(vol.backupName)

	// list directory having schedule/backup name as prefix
	dirs, err := p.cl.ListKeys(p.cl.BackupPathPrefix(scheduleName), clouduploader.KeyDirectory)
	if err != nil {
		return snapList, errors.Wrapf(err, "failed to get list of directory")
	}

	for _, dir := range dirs {
		// list files for dir having volume name as prefix
		files, err := p.cl.ListKeys(dir+p.cl.FilePathPrefix(vol.snapshotTag), clouduploader.KeyFile)
		if err != nil {
			return snapList, errors.Wrapf(err, "failed to get list of snapshot file at path=%v", dir)
		}

		if len(files) != 0 {
			// snapshot exist in the backup directory

			// add backup name from dir path to snapList
			s := strings.Split(dir, "/")

			// dir will contain path with trailing '/', example: 'backups/b-0/'
			snapList = append(snapList, s[len(s)-2])
		}
	}
	return snapList, nil
}
