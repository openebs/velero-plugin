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
	"fmt"
	"strings"
	"time"

	cstorv1 "github.com/openebs/api/pkg/apis/cstor/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cVRPVLabel                 = "openebs.io/persistent-volume"
	restoreCompletedAnnotation = "openebs.io/restore-completed"
)

var validCvrStatusesWithoutError = []string{
	string(cstorv1.CVRStatusOnline),
	string(cstorv1.CVRStatusDegraded),
}

var validCvrStatuses = []string{
	string(cstorv1.CVRStatusOnline),
	string(cstorv1.CVRStatusError),
	string(cstorv1.CVRStatusDegraded),
}

// CVRWaitCount control time limit for waitForAllCVR
var CVRWaitCount = 100

// CVRCheckInterval defines amount of delay for CVR check
var CVRCheckInterval = 5 * time.Second

// waitForAllCVRs will ensure that all CVR related to
// the given volume is created
func (p *Plugin) waitForAllCVRs(vol *Volume) error {
	return p.waitForAllCVRsToBeInValidStatus(vol, validCvrStatuses)
}

// waitForTargetIpToBeSetInAllCVRs will ensure that all CVR had
// target ip set in zfs
func (p *Plugin) waitForTargetIpToBeSetInAllCVRs(vol *Volume) error {
	return p.waitForAllCVRsToBeInValidStatus(vol, validCvrStatusesWithoutError)
}

func (p *Plugin) waitForAllCVRsToBeInValidStatus(vol *Volume, statuses []string) error {
	replicaCount := p.getCVRCount(vol.volname, vol.isCSIVolume)

	if replicaCount == -1 {
		return errors.Errorf("Failed to fetch replicaCount for volume{%s}", vol.volname)
	}

	if vol.isCSIVolume {
		return p.waitForCSIBasedCVRs(vol, replicaCount, statuses)
	}
	return p.waitFoNonCSIBasedCVRs(vol, replicaCount, statuses)
}

// waitFoNonCSIBasedCVRs will ensure that all CVRs related to
// given non CSI based volume is created
func (p *Plugin) waitFoNonCSIBasedCVRs(vol *Volume, replicaCount int, statuses []string) error {
	for cnt := 0; cnt < CVRWaitCount; cnt++ {
		cvrList, err := p.OpenEBSClient.
			OpenebsV1alpha1().
			CStorVolumeReplicas(p.namespace).
			List(metav1.ListOptions{
				LabelSelector: cVRPVLabel + "=" + vol.volname,
			})
		if err != nil {
			return errors.Errorf("Failed to fetch CVR for volume=%s %s", vol.volname, err)
		}
		if len(cvrList.Items) != replicaCount {
			time.Sleep(CVRCheckInterval)
			continue
		}
		cvrCount := 0
		for _, cvr := range cvrList.Items {
			if contains(statuses, string(cvr.Status.Phase)) {
				cvrCount++
			}
		}
		if cvrCount == replicaCount {
			return nil
		}
		time.Sleep(CVRCheckInterval)
	}
	return errors.Errorf("CVR for volume{%s} are not ready!", vol.volname)
}

// waitForCSIBasedCVRs will ensure that all CVRs related to
// the given CSI volume is created.
func (p *Plugin) waitForCSIBasedCVRs(vol *Volume, replicaCount int, statuses []string) error {
	for cnt := 0; cnt < CVRWaitCount; cnt++ {
		cvrList, err := p.OpenEBSAPIsClient.
			CstorV1().
			CStorVolumeReplicas(p.namespace).
			List(metav1.ListOptions{
				LabelSelector: cVRPVLabel + "=" + vol.volname,
			})
		if err != nil {
			return errors.Errorf("Failed to fetch CVR for volume=%s %s", vol.volname, err)
		}

		if len(cvrList.Items) != replicaCount {
			time.Sleep(CVRCheckInterval)
			continue
		}

		cvrCount := 0
		for _, cvr := range cvrList.Items {
			if contains(statuses, string(cvr.Status.Phase)) {
				cvrCount++
			}
		}
		if cvrCount == replicaCount {
			return nil
		}
		time.Sleep(CVRCheckInterval)
	}
	return errors.Errorf("CVR for volume{%s} are not ready!", vol.volname)
}

// getCVRCount returns the number of CVR for a given volume
func (p *Plugin) getCVRCount(volname string, isCSIVolume bool) int {
	// For CSI based volume, CVR of v1 is used.
	if isCSIVolume {
		// If the volume is CSI based, then CVR V1 is used.
		obj, err := p.OpenEBSAPIsClient.
			CstorV1().
			CStorVolumes(p.namespace).
			Get(volname, metav1.GetOptions{})
		if err != nil {
			p.Log.Errorf("Failed to fetch cstorVolume.. %s", err)
			return -1
		}

		return obj.Spec.ReplicationFactor
	}
	// For non CSI based volume, CVR of v1alpha1 is used.
	obj, err := p.OpenEBSClient.
		OpenebsV1alpha1().
		CStorVolumes(p.namespace).
		Get(volname, metav1.GetOptions{})
	if err != nil {
		p.Log.Errorf("Failed to fetch cstorVolume.. %s", err)
		return -1
	}
	return obj.Spec.ReplicationFactor
}

// markCVRsAsRestoreCompleted wait for all CVRs to be ready, then marks CVRs are restore completed
// and wait for CVRs to be in non-error state
func (p *Plugin) markCVRsAsRestoreCompleted(vol *Volume) error {
	p.Log.Infof("Waiting for all CVRs to be ready")
	if err := p.waitForAllCVRs(vol); err != nil {
		return err
	}

	p.Log.Infof("Waiting for all CVRs to be ready")
	if err := p.markRestoreAsCompleted(vol); err != nil {
		p.Log.Errorf("Failed to mark restore as completed : %s", err)
		return err
	}

	p.Log.Infof("Waiting for target ip to be set ono all CVRs")
	if err := p.waitForTargetIpToBeSetInAllCVRs(vol); err != nil {
		return err
	}

	return nil
}

// markRestoreAsCompleted will mark CVRs that the restore was completed
func (p *Plugin) markRestoreAsCompleted(vol *Volume) error {
	if vol.isCSIVolume {
		return p.markRestoreAsCompletedForCSIBasedCVRs(vol)
	}
	return p.markRestoreAsCompletedForNonCSIBasedCVRs(vol)
}

func (p *Plugin) markRestoreAsCompletedForCSIBasedCVRs(vol *Volume) error {
	replicas := p.OpenEBSAPIsClient.CstorV1().
		CStorVolumeReplicas(p.namespace)

	cvrList, err := replicas.
		List(metav1.ListOptions{
			LabelSelector: cVRPVLabel + "=" + vol.volname,
		})

	if err != nil {
		return errors.Errorf("Failed to fetch CVR for volume=%s %s", vol.volname, err)
	}

	var errs []string
	for _, cvr := range cvrList.Items {
		p.Log.Infof("Updating CVRs %s", cvr.Name)

		cvr.Annotations[restoreCompletedAnnotation] = "true"
		_, err := replicas.Update(&cvr)

		if err != nil {
			p.Log.Warnf("could not update CVR %s", cvr.Name)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}

	return nil
}

func (p *Plugin) markRestoreAsCompletedForNonCSIBasedCVRs(vol *Volume) error {
	replicas := p.OpenEBSClient.OpenebsV1alpha1().
		CStorVolumeReplicas(p.namespace)

	cvrList, err := replicas.
		List(metav1.ListOptions{
			LabelSelector: cVRPVLabel + "=" + vol.volname,
		})

	if err != nil {
		return errors.Errorf("Failed to fetch CVR for volume=%s %s", vol.volname, err)
	}

	var errs []string
	for _, cvr := range cvrList.Items {
		p.Log.Infof("Updating CVRs %s", cvr.Name)

		cvr.Annotations[restoreCompletedAnnotation] = "true"
		_, err := replicas.Update(&cvr)

		if err != nil {
			p.Log.Warnf("could not update CVR %s", cvr.Name)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}

	return nil
}
