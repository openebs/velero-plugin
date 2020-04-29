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

package openebs

import (
	"fmt"

	k8s "github.com/openebs/velero-plugin/tests/k8s"
	corev1 "k8s.io/api/core/v1"
)

const (
	mayaAPIPodLabel = "openebs.io/component-name=maya-apiserver"
	cstorPodLabel   = "app=cstor-pool"
	pvcPodLabel     = "openebs.io/target=cstor-target"
)

// DumpLogs will dump openebs logs
func (c *ClientSet) DumpLogs() {
	mayaPod := c.getMayaAPIServerPodName()
	spcPod := c.getSPCPodName()
	pvcPod := c.getPVCPodName()

	for _, v := range mayaPod {
		if err := k8s.Client.DumpLogs(OpenEBSNs, v[0], v[1]); err != nil {
			fmt.Printf("Failed to dump maya-apiserver logs err=%s\n", err)
		}
	}
	for _, v := range spcPod {
		if err := k8s.Client.DumpLogs(OpenEBSNs, v[0], v[1]); err != nil {
			fmt.Printf("Failed to dump cstor pod logs err=%s\n", err)
		}
	}
	for _, v := range pvcPod {
		if err := k8s.Client.DumpLogs(OpenEBSNs, v[0], v[1]); err != nil {
			fmt.Printf("Failed to dump target pod logs err=%s\n", err)
		}
	}
}

// getMayaAPIServerPodName return Maya-API server pod name and container
// {{"pod1","container1"},{"pod2","container2"},}
func (c *ClientSet) getMayaAPIServerPodName() [][]string {
	podList, err := k8s.Client.GetPodList(OpenEBSNs,
		mayaAPIPodLabel,
	)
	if err != nil {
		return [][]string{}
	}
	return getPodContainerList(podList)
}

// getSPCPodName return SPC pod name and container
// {{"pod1","container1"},{"pod2","container2"},}
func (c *ClientSet) getSPCPodName() [][]string {
	podList, err := k8s.Client.GetPodList(OpenEBSNs,
		cstorPodLabel,
	)
	if err != nil {
		return [][]string{}
	}
	return getPodContainerList(podList)
}

// getPVCPodName return PVC pod name and container
// {{"pod1","container1"},{"pod2","container2"},}
func (c *ClientSet) getPVCPodName() [][]string {
	podList, err := k8s.Client.GetPodList(OpenEBSNs,
		pvcPodLabel,
	)
	if err != nil {
		return [][]string{}
	}
	return getPodContainerList(podList)
}

// returns {{"pod1","container1"},{"pod2","container2"},}
func getPodContainerList(podList *corev1.PodList) [][]string {
	pod := make([][]string, 0)

	for _, p := range podList.Items {
		for _, c := range p.Spec.Containers {
			pod = append(pod, []string{p.Name, c.Name})
		}
	}
	return pod
}
