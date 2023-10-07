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
	"context"
	"encoding/json"
	"sort"

	uuid "github.com/gofrs/uuid"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PvClonePrefix  prefix for clone volume in case restore from local backup
	PvClonePrefix = "cstor-clone-"
)

func (p *Plugin) updateVolCASInfo(data []byte, volumeID string) error {
	var cas v1alpha1.CASVolume

	vol := p.volumes[volumeID]
	if vol == nil {
		return errors.Errorf("Volume{%s} not found in volume list", volumeID)
	}

	if !vol.isCSIVolume {
		err := json.Unmarshal(data, &cas)
		if err != nil {
			return err
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
	//NOTE: As of now no need to handle restore response for cStor CSI volumes
	return nil
}

// restoreVolumeFromCloud restore remote snapshot for the given volume
// Note: cstor snapshots are incremental in nature, so restore will be executed
// from base snapshot to incremental snapshot 'vol.backupName' if p.restoreAllSnapshots is set
// else restore will be performed for the given backup only.
func (p *Plugin) restoreVolumeFromCloud(vol *Volume, targetBackupName string) error {
	var (
		snapshotList []string
		err          error
	)

	p.Log.Info("Restoring volume data from cloud")
	if p.restoreAllSnapshots {
		// We are restoring from base backup to targeted Backup
		snapshotList, err = p.cl.GetSnapListFromCloud(vol.snapshotTag, p.getScheduleName(targetBackupName))
		if err != nil {
			return err
		}
	} else {
		// We are restoring only given backup
		snapshotList = []string{targetBackupName}
	}

	if !contains(snapshotList, targetBackupName) {
		return errors.Errorf("Targeted backup=%s not found in snapshot list", targetBackupName)
	}

	// snapshots are created using timestamp, we need to sort it in ascending order
	sort.Strings(snapshotList)

	for _, snap := range snapshotList {
		// Check if snapshot file exists or not.
		// There is a possibility where only PVC file exists,
		// in case of failed/partially-failed backup, but not snapshot file.
		exists, err := p.cl.FileExists(vol.snapshotTag, snap)
		if err != nil {
			p.Log.Errorf("Failed to check remote snapshot=%s, skipping restore of this snapshot, err=%s", snap, err)
			continue
		}

		// If the snapshot doesn't exist, skip the restore for that snapshot.
		// Since the snapshots are incremental, we need to continue to restore for the next snapshot.
		if !exists {
			p.Log.Warningf("Remote snapshot=%s doesn't exist, skipping restore of this snapshot", snap)
			continue
		}

		p.Log.Infof("Restoring snapshot=%s", snap)

		vol.backupName = snap

		err = p.restoreSnapshotFromCloud(vol)
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

	ret := p.cl.Download(filename, CstorRestorePort)
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
		Get(context.TODO(), volumeID, metav1.GetOptions{})
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

// contains return true if given target string exists in slice s
func contains(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}

	return false
}

// backupPV perform backup for given volume's PV
func (p *Plugin) backupPV(volumeID string) error {
	vol := p.volumes[volumeID]

	bkpPv, err := p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(context.TODO(), vol.volname, metav1.GetOptions{})
	if err != nil {
		p.Log.Errorf("Error fetching PV(%s): %s", vol.volname, err.Error())
		return errors.New("failed to fetch PV")
	}

	data, err := json.MarshalIndent(bkpPv, "", "\t")
	if err != nil {
		return errors.New("error doing json parsing")
	}

	filename := p.cl.GenerateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("error creating remote file name for pvc backup")
	}

	if ok := p.cl.Write(data, filename+".pv"); !ok {
		return errors.New("failed to upload PV")
	}

	return nil
}

// restorePV create PV for given volume name
func (p *Plugin) restorePV(volumeID, snapName string) error {
	_, err := p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(context.TODO(), volumeID, metav1.GetOptions{})
	if err == nil {
		p.Log.Infof("PV=%s already exists, skip restore", volumeID)
		return nil
	}
	if !k8serrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to get PV=%s", volumeID)
	}

	pv, err := p.downloadPV(volumeID, snapName)
	if err != nil {
		return errors.Wrapf(err, "failed to download pv")
	}

	// Add annotation PVCreatedByKey, with value 'restore' to PV
	pv.Annotations = make(map[string]string)
	pv.Annotations[v1alpha1.PVCreatedByKey] = "restore"
	pv.ManagedFields = nil
	pv.Finalizers = nil
	if pv.Spec.ClaimRef != nil {
		pv.Spec.ClaimRef.ResourceVersion = ""
		pv.Spec.ClaimRef.UID = ""
	}
	pv.CreationTimestamp = metav1.Time{}
	pv.ResourceVersion = ""
	pv.UID = ""
	pv.Status = v1.PersistentVolumeStatus{}

	_, err = p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Create(context.TODO(), pv, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create PV=%s", pv.Name)
		}
		p.Log.Infof("PV=%s already exists, skip restore", pv.Name)
	}

	return nil
}

func (p *Plugin) downloadPV(volumeID, snapName string) (*v1.PersistentVolume, error) {
	pv := &v1.PersistentVolume{}

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)

	data, ok := p.cl.Read(filename + ".pv")
	if !ok {
		return nil, errors.Errorf("failed to download PV file=%s", filename+".pv")
	}

	if err := json.Unmarshal(data, pv); err != nil {
		return nil, errors.Errorf("failed to decode pv file=%s", filename+".pv")
	}

	return pv, nil
}
