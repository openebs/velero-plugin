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
	"time"

	"github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CVRWaitCount control time limit for waitForAllCVR
var CVRWaitCount = 100

// CVRCheckInterval defines amount of delay for CVR check
var CVRCheckInterval = 5 * time.Second

// waitForAllCVR will ensure that all CVR related to
// given volumes are created
func (p *Plugin) waitForAllCVR(vol *Volume) error {
	replicaCount := p.getCVRCount(vol.volname)
	if replicaCount == -1 {
		return errors.Errorf("Failed to fetch replicaCount for volume{%s}", vol.volname)
	}

	for cnt := 0; cnt < CVRWaitCount; cnt++ {
		cvrList, err := p.OpenEBSClient.
			OpenebsV1alpha1().
			CStorVolumeReplicas(p.namespace).
			List(metav1.ListOptions{
				LabelSelector: "openebs.io/persistent-volume=" + vol.volname,
			})
		if err != nil {
			return errors.Errorf("Failed to fetch CVR.. %s", err)
		}

		if len(cvrList.Items) != replicaCount {
			time.Sleep(CVRCheckInterval)
			continue
		}

		cvrCount := 0
		for _, cvr := range cvrList.Items {
			if cvr.Status.Phase == v1alpha1.CVRStatusOnline ||
				cvr.Status.Phase == v1alpha1.CVRStatusError ||
				cvr.Status.Phase == v1alpha1.CVRStatusDegraded {
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

// getCVRCount returns the number of CVR for given volume
func (p *Plugin) getCVRCount(volname string) int {
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
