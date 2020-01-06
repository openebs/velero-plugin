package cstor

import (
	"encoding/json"

	uuid "github.com/gofrs/uuid"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		return errors.New("Failed to restore snapshot")
	}

	if vol.restoreStatus != v1alpha1.RSTCStorStatusDone {
		return errors.Errorf("Failed to restore.. status {%s}", vol.restoreStatus)
	}

	return nil
}

func (p *Plugin) generateRestorePVName(volumeID string) (string, error) {
	pv, err := p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(volumeID, metav1.GetOptions{})
	if err != nil {
		if apierror.IsNotFound(err) {
			return volumeID, nil
		}
		return "", errors.Wrapf(err, "Error checking if PV with same name exist")
	}

	if pv.Spec.ClaimRef == nil {
		p.Log.Infof("PV %s is not claimed.. using the same PV for restore", volumeID)
		return volumeID, nil
	}

	nuuid, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrapf(err, "Error generating uuid for PV rename")
	}

	oldVolumeID, volumeID := volumeID, "cstor-clone-"+nuuid.String()
	p.Log.Infof("Renaming PV %s to %s", oldVolumeID, volumeID)
	return volumeID, nil
}
