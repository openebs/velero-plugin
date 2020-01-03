package cstor

import (
	"encoding/json"

	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
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
