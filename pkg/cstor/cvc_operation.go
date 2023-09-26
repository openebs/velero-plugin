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

package cstor

import (
	"encoding/json"
	"fmt"

	cstorv1 "github.com/openebs/api/v2/pkg/apis/cstor/v1"
	maya "github.com/openebs/cstor-csi/pkg/utils"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// (Kasakaze)todo: Determine whether it is csiVolume, if so, cvc must be backed up
func (p *Plugin) backupCVC(volumeID string) error {
	vol := p.volumes[volumeID]

	bkpCvc, err := maya.GetVolume(volumeID)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return errors.Cause(err)
		}
		p.Log.Warnf("failed to get cvc, skip. %v", err)
		return nil
	}

	data, err := json.MarshalIndent(bkpCvc, "", "\t")
	if err != nil {
		return errors.New("error doing json parsing")
	}

	// pv backup file name
	filename := p.cl.GenerateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("error creating remote file name for pvc backup")
	}
	if ok := p.cl.Write(data, filename+".cvc"); !ok {
		return errors.New("failed to upload CVC")
	}

	return nil
}

// restoreCVC create CVC for given volume name
// (Kasakaze)todo: Determine whether it is csiVolume, if so, cvc must be restored
func (p *Plugin) restoreCVC(volumeID, pvcName, pvcNamespace, snapName string) error {
	// verify if the volume has already been created
	cvc, err := maya.GetVolume(volumeID)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return errors.Cause(err)
		}
	}
	if err == nil && cvc != nil && cvc.DeletionTimestamp == nil {
		p.Log.Warn("cvc already exists, don't provision volume")
		return nil
	}

	p.Log.Info("cvc does not exist, download and provision")
	rcvc, err := p.downloadCVC(volumeID, snapName)
	if err != nil {
		p.Log.Warnf("failed to download cvc, skip. %v", err)
		return nil
	}

	var (
		size, _    = rcvc.Spec.Capacity.Storage().AsInt64()
		rCount     = fmt.Sprint(rcvc.Spec.Provision.ReplicaCount)
		cspcName   = rcvc.ObjectMeta.Labels["openebs.io/cstor-pool-cluster"]
		snapshotID = ""
		// (Kasakaze)todo: If the data is migrated to another cluster, the nodeID may not be the same
		nodeID     = rcvc.Publish.NodeID
		policyName = rcvc.ObjectMeta.Labels["openebs.io/volume-policy"]
	)

	err = maya.ProvisionVolume(size, volumeID, rCount,
		cspcName, snapshotID,
		nodeID, policyName, pvcName, pvcNamespace)
	if err != nil {
		return errors.Cause(err)
	}

	return nil
}

func (p *Plugin) downloadCVC(volumeID, snapName string) (*cstorv1.CStorVolumeConfig, error) {
	cvc := &cstorv1.CStorVolumeConfig{}

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)
	filename += ".cvc"
	data, ok := p.cl.Read(filename)
	if !ok {
		return nil, errors.Errorf("failed to download CVC file=%s", filename)
	}

	if err := json.Unmarshal(data, cvc); err != nil {
		return nil, errors.Errorf("failed to decode CVC file=%s", filename)
	}

	return cvc, nil
}
