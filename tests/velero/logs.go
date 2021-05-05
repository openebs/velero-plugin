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

package velero

import (
	"fmt"
	"os"
	"time"

	"github.com/openebs/velero-plugin/tests/k8s"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	log "github.com/vmware-tanzu/velero/pkg/cmd/util/downloadrequest"
	corev1 "k8s.io/api/core/v1"
)

// DumpBackupLogs dump logs of given backup on stdout
func (c *ClientSet) DumpBackupLogs(backupName string) error {
	return log.Stream(c.VeleroV1(),
		VeleroNamespace,
		backupName,
		v1.DownloadTargetKindBackupLog,
		os.Stdout,
		time.Minute, false, "")
}

// DumpLogs dump logs of velero pod on stdout
func (c *ClientSet) DumpLogs() {
	veleroPod := c.getPodName()

	for _, v := range veleroPod {
		if err := k8s.Client.DumpLogs(VeleroNamespace, v[0], v[1]); err != nil {
			fmt.Printf("Failed to dump velero logs, err=%s\n", err)
		}
	}
}

// getPodName return velero pod name and container name
// {{"pod_1","container_1"},{"pod_2","container_2"},}
func (c *ClientSet) getPodName() [][]string {
	podList, err := k8s.Client.GetPodList(VeleroNamespace,
		"deploy=velero",
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
