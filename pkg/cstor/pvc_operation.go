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
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCWaitCount control time limit for createPVC
var PVCWaitCount = 100

// PVCCheckInterval defines amount of delay for PVC bound check
var PVCCheckInterval = 5 * time.Second

// backupPVC perform backup for given volume's PVC
func (p *Plugin) backupPVC(volumeID string) error {
	vol := p.volumes[volumeID]
	var bkpPvc *v1.PersistentVolumeClaim

	pvcs, err := p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(vol.namespace).
		List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorf("Error fetching PVC list : %s", err.Error())
		return errors.New("Failed to fetch PVC list")
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == vol.volname {
			bkpPvc = &pvc
			break
		}
	}

	if bkpPvc == nil {
		p.Log.Errorf("Failed to find PVC for PV{%s}", vol.volname)
		return errors.Errorf("Failed to find PVC for volume{%s}", vol.volname)
	}

	bkpPvc.ResourceVersion = ""
	bkpPvc.SelfLink = ""
	if bkpPvc.Spec.StorageClassName == nil || len(*bkpPvc.Spec.StorageClassName) == 0 {
		sc := bkpPvc.Annotations[v1.BetaStorageClassAnnotation]
		bkpPvc.Spec.StorageClassName = &sc
	}

	bkpPvc.Annotations = nil
	bkpPvc.UID = ""
	bkpPvc.Spec.VolumeName = ""

	data, err := json.MarshalIndent(bkpPvc, "", "\t")
	if err != nil {
		return errors.New("Error doing json parsing")
	}

	filename := p.cl.GenerateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("Error creating remote file name for pvc backup")
	}

	if ok := p.cl.Write(data, filename+".pvc"); !ok {
		return errors.New("Failed to upload PVC")
	}

	return nil
}

// createPVC create PVC for given volume name
func (p *Plugin) createPVC(volumeID, snapName string) (*Volume, error) {
	pvc := &v1.PersistentVolumeClaim{}
	var vol *Volume
	var data []byte
	var ok bool

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)
	if filename == "" {
		return nil, errors.New("Error creating remote file name for pvc backup")
	}

	if data, ok = p.cl.Read(filename + ".pvc"); !ok {
		return nil, errors.New("Failed to download PVC")
	}

	if err := json.Unmarshal(data, pvc); err != nil {
		return nil, errors.New("Failed to decode pvc")
	}

	newVol, err := p.getVolumeFromPVC(*pvc)
	if err == nil {
		newVol.backupName = snapName
		return newVol, nil
	}

	pvc.Annotations = make(map[string]string)
	pvc.Annotations["openebs.io/created-through"] = "restore"
	rpvc, err := p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(pvc.Namespace).
		Create(pvc)
	if err != nil {
		return nil, errors.Errorf("Failed to create PVC : %s", err.Error())
	}

	for cnt := 0; cnt < PVCWaitCount; cnt++ {
		pvc, err = p.K8sClient.
			CoreV1().
			PersistentVolumeClaims(rpvc.Namespace).
			Get(rpvc.Name, metav1.GetOptions{})
		if err != nil || pvc.Status.Phase == v1.ClaimLost {
			if err := p.K8sClient.
				CoreV1().
				PersistentVolumeClaims(pvc.Namespace).
				Delete(rpvc.Name, nil); err != nil {
				p.Log.Warnf("Failed to delete pvc {%s} : %s", rpvc.Name, err.Error())
			}
			return nil, errors.Errorf("Failed to create PVC : %s", err.Error())
		}
		if pvc.Status.Phase == v1.ClaimBound {
			p.Log.Infof("PVC(%v) created..", pvc.Name)
			vol = &Volume{
				volname:    pvc.Spec.VolumeName,
				namespace:  pvc.Namespace,
				backupName: snapName,
				casType:    *pvc.Spec.StorageClassName,
			}
			break
		}
		time.Sleep(PVCCheckInterval)
	}

	if vol == nil {
		return nil, errors.Errorf("PVC{%s} is not bounded!", rpvc.Name)
	}

	if err = p.waitForAllCVR(vol); err != nil {
		return nil, err
	}
	return vol, nil
}

// getVolumeFromPVC returns volume info for given PVC if PVC is in bound state
func (p *Plugin) getVolumeFromPVC(pvc v1.PersistentVolumeClaim) (*Volume, error) {
	rpvc, err := p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(pvc.Namespace).
		Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Errorf("PVC{%s} does not exist", pvc.Name)
	}

	if rpvc.Status.Phase == v1.ClaimLost {
		p.Log.Errorf("PVC{%s} is not bound yet!", rpvc.Name)
		panic(errors.Errorf("PVC{%s} is not bound yet", rpvc.Name))
	} else {
		vol := &Volume{
			volname:   rpvc.Spec.VolumeName,
			namespace: rpvc.Namespace,
			casType:   *rpvc.Spec.StorageClassName,
		}

		if err = p.waitForAllCVR(vol); err != nil {
			return nil, err
		}
		return vol, nil
	}
}
