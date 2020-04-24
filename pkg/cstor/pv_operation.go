package cstor

import (
	"encoding/json"

	uuid "github.com/gofrs/uuid"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (p *Plugin) restoreVolumeFromCloud(vol *Volume) error {
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

	vol := &Volume{
		volname:      clonePvName,
		srcVolname:   pv.Name,
		backupName:   snapName,
		storageClass: pv.Spec.StorageClassName,
		size:         pv.Spec.Capacity[v1.ResourceStorage],
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

	p.volumes[vol.volname] = vol

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
