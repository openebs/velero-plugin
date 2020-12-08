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

	"github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/openebs/velero-plugin/pkg/velero"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// PVCWaitCount control time limit for createPVC
	PVCWaitCount = 100

	// NamespaceCreateTimeout defines timeout for namespace creation
	NamespaceCreateTimeout = 5 * time.Minute

	// PVCCheckInterval defines amount of delay for PVC bound check
	PVCCheckInterval = 5 * time.Second
)

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
		return errors.New("failed to fetch PVC list")
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == vol.volname {
			fPVC := pvc
			bkpPvc = &fPVC
			break
		}
	}

	if bkpPvc == nil {
		p.Log.Errorf("Failed to find PVC for PV{%s}", vol.volname)
		return errors.Errorf("Failed to find PVC for volume{%s}", vol.volname)
	}

	bkpPvc.ResourceVersion = ""
	bkpPvc.SelfLink = ""
	if bkpPvc.Spec.StorageClassName == nil || *bkpPvc.Spec.StorageClassName == "" {
		sc := bkpPvc.Annotations[v1.BetaStorageClassAnnotation]
		bkpPvc.Spec.StorageClassName = &sc
	}

	bkpPvc.Annotations = nil
	bkpPvc.UID = ""
	bkpPvc.Spec.VolumeName = ""

	data, err := json.MarshalIndent(bkpPvc, "", "\t")
	if err != nil {
		return errors.New("error doing json parsing")
	}

	filename := p.cl.GenerateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("error creating remote file name for pvc backup")
	}

	if ok := p.cl.Write(data, filename+".pvc"); !ok {
		return errors.New("failed to upload PVC")
	}

	return nil
}

// createPVC create PVC for given volume name
func (p *Plugin) createPVC(volumeID, snapName string) (*Volume, error) {
	var vol *Volume

	pvc, err := p.downloadPVC(volumeID, snapName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download pvc")
	}

	targetedNs, err := velero.GetRestoreNamespace(pvc.Namespace, snapName, p.Log)
	if err != nil {
		return nil, err
	}

	err = p.EnsureNamespaceOrCreate(targetedNs)
	if err != nil {
		return nil, errors.Wrapf(err, "error verifying namespace")
	}

	pvc.Namespace = targetedNs

	newVol, err := p.getVolumeFromPVC(*pvc)
	if err != nil {
		return nil, err
	}

	if newVol != nil {
		newVol.backupName = snapName
		newVol.snapshotTag = volumeID
		return newVol, nil
	}

	p.Log.Infof("Creating PVC for volumeID:%s snapshot:%s in namespace=%s", volumeID, snapName, targetedNs)

	pvc.Annotations = make(map[string]string)
	// Add annotation PVCreatedByKey, with value 'restore' to PVC
	// So that Maya-APIServer skip updating target IPAddress in CVR
	pvc.Annotations[v1alpha1.PVCreatedByKey] = "restore"
	rpvc, err := p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(pvc.Namespace).
		Create(pvc)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create PVC=%s/%s", pvc.Namespace, pvc.Name)
	}

	for cnt := 0; cnt < PVCWaitCount; cnt++ {
		pvc, err = p.K8sClient.
			CoreV1().
			PersistentVolumeClaims(rpvc.Namespace).
			Get(rpvc.Name, metav1.GetOptions{})
		if err != nil || pvc.Status.Phase == v1.ClaimLost {
			if err = p.K8sClient.
				CoreV1().
				PersistentVolumeClaims(rpvc.Namespace).
				Delete(rpvc.Name, nil); err != nil {
				p.Log.Warnf("Failed to delete pvc {%s/%s} : %s", rpvc.Namespace, rpvc.Name, err.Error())
			}
			return nil, errors.Wrapf(err, "failed to create PVC=%s/%s", rpvc.Namespace, rpvc.Name)
		}
		if pvc.Status.Phase == v1.ClaimBound {
			p.Log.Infof("PVC(%v) created..", pvc.Name)
			vol = &Volume{
				volname:      pvc.Spec.VolumeName,
				snapshotTag:  volumeID,
				namespace:    pvc.Namespace,
				backupName:   snapName,
				storageClass: *pvc.Spec.StorageClassName,
			}
			p.volumes[vol.volname] = vol
			break
		}
		time.Sleep(PVCCheckInterval)
	}

	if vol == nil {
		return nil, errors.Errorf("PVC{%s/%s} is not bounded!", rpvc.Namespace, rpvc.Name)
	}

	// check for created volume type
	pv, err := p.getPV(vol.volname)

	if err != nil {
		p.Log.Errorf("Failed to get PV{%s}", vol.volname)
		return nil, errors.Wrapf(err, "failed to get pv=%s", vol.volname)
	}

	vol.isCSIVolume = isCSIPv(*pv)
	if err = p.waitForAllCVRs(vol); err != nil {
		return nil, err
	}

	// CVRs are created and updated, now we can remove the annotation 'PVCreatedByKey' from PVC
	if err = p.removePVCAnnotationKey(pvc, v1alpha1.PVCreatedByKey); err != nil {
		p.Log.Warningf("Failed to remove restore annotation from PVC=%s/%s err=%s", pvc.Namespace, pvc.Name, err)
		return nil, errors.Wrapf(err,
			"failed to clear restore-annotation=%s from PVC=%s/%s",
			v1alpha1.PVCreatedByKey, pvc.Namespace, pvc.Name,
		)
	}
	return vol, nil
}

// nolint: unused
func (p *Plugin) getPVCInfo(volumeID, snapName string) (*Volume, error) {
	pvc := &v1.PersistentVolumeClaim{}
	var vol *Volume
	var data []byte
	var ok bool

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)
	if filename == "" {
		return nil, errors.New("error creating remote file name for pvc backup")
	}

	if data, ok = p.cl.Read(filename + ".pvc"); !ok {
		return nil, errors.New("failed to download PVC")
	}

	if err := json.Unmarshal(data, pvc); err != nil {
		return nil, errors.New("failed to decode pvc")
	}

	vol = &Volume{
		volname:      volumeID,
		snapshotTag:  volumeID,
		backupName:   snapName,
		storageClass: *pvc.Spec.StorageClassName,
		size:         pvc.Spec.Resources.Requests[v1.ResourceStorage],
	}
	p.volumes[vol.volname] = vol
	return vol, nil
}

// getVolumeFromPVC returns volume info for given PVC if PVC is in bound state
func (p *Plugin) getVolumeFromPVC(pvc v1.PersistentVolumeClaim) (*Volume, error) {
	rpvc, err := p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(pvc.Namespace).
		Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed to fetch PVC{%s}", pvc.Name)
	}

	if rpvc.Status.Phase == v1.ClaimLost {
		p.Log.Errorf("PVC{%s} is not bound yet!", rpvc.Name)
		return nil, errors.Errorf("pvc{%s} is not bound", rpvc.Name)
	}
	// check for created volume type
	pv, err := p.getPV(rpvc.Spec.VolumeName)
	if err != nil {
		p.Log.Errorf("Failed to get PV{%s}", rpvc.Spec.VolumeName)
		return nil, errors.Wrapf(err, "failed to get pv=%s", rpvc.Spec.VolumeName)
	}
	isCSIVolume := isCSIPv(*pv)
	vol := &Volume{
		volname:      rpvc.Spec.VolumeName,
		snapshotTag:  rpvc.Spec.VolumeName,
		namespace:    rpvc.Namespace,
		storageClass: *rpvc.Spec.StorageClassName,
		isCSIVolume:  isCSIVolume,
	}
	p.volumes[vol.volname] = vol

	if err = p.waitForAllCVRs(vol); err != nil {
		return nil, errors.Wrapf(err, "cvr not ready")
	}

	// remove the annotation 'PVCreatedByKey' from PVC
	// There might be chances of stale PVCreatedByKey annotation in PVC
	if err = p.removePVCAnnotationKey(rpvc, v1alpha1.PVCreatedByKey); err != nil {
		p.Log.Warningf("Failed to remove restore annotation from PVC=%s/%s err=%s", rpvc.Namespace, rpvc.Name, err)
		return nil, errors.Wrapf(err,
			"failed to clear restore-annotation=%s from PVC=%s/%s",
			v1alpha1.PVCreatedByKey, rpvc.Namespace, rpvc.Name,
		)
	}

	return vol, nil
}

func (p *Plugin) downloadPVC(volumeID, snapName string) (*v1.PersistentVolumeClaim, error) {
	pvc := &v1.PersistentVolumeClaim{}

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)

	data, ok := p.cl.Read(filename + ".pvc")
	if !ok {
		return nil, errors.Errorf("failed to download PVC file=%s", filename+".pvc")
	}

	if err := json.Unmarshal(data, pvc); err != nil {
		return nil, errors.Errorf("failed to decode pvc file=%s", filename+".pvc")
	}

	return pvc, nil
}

// removePVCAnnotationKey remove the given annotation key from the PVC and update it
func (p *Plugin) removePVCAnnotationKey(pvc *v1.PersistentVolumeClaim, annotationKey string) error {
	var err error

	if pvc.Annotations == nil {
		return nil
	}

	delete(pvc.Annotations, annotationKey)

	_, err = p.K8sClient.
		CoreV1().
		PersistentVolumeClaims(pvc.Namespace).
		Update(pvc)
	return err
}

// EnsureNamespaceOrCreate ensure that given namespace exists and ready
// - If namespace exists and ready to use then it will return nil
// - If namespace is in terminating state then function will wait for ns removal and re-create it
// - If namespace doesn't exist then function will create it
func (p *Plugin) EnsureNamespaceOrCreate(ns string) error {
	checkNs := func(namespace string) (bool, error) {
		var isNsReady bool

		err := wait.PollImmediate(time.Second, NamespaceCreateTimeout, func() (bool, error) {
			p.Log.Debugf("Checking namespace=%s", namespace)

			obj, err := p.K8sClient.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					return false, err
				}

				// namespace doesn't exist
				return true, nil
			}

			if obj.GetDeletionTimestamp() != nil || obj.Status.Phase == v1.NamespaceTerminating {
				// will wait till namespace get deleted
				return false, nil
			}

			if obj.Status.Phase == v1.NamespaceActive {
				isNsReady = true
			}

			return isNsReady, nil
		})

		return isNsReady, err
	}

	isReady, err := checkNs(ns)
	if err != nil {
		return errors.Wrapf(err, "failed to check namespace")
	}

	if isReady {
		return nil
	}

	// namespace doesn't exist, create it
	nsObj := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	p.Log.Infof("Creating namespace=%s", ns)

	_, err = p.K8sClient.CoreV1().Namespaces().Create(nsObj)
	if err != nil {
		return errors.Wrapf(err, "failed to create namespace")
	}

	return nil
}
