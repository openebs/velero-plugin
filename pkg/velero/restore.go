/*
Copyright 2020 The OpenEBS Authors.

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
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetRestoreNamespace return the namespace mapping for the given namespace
// if namespace mapping not found then it will return the same namespace in which backup was created
// if namespace mapping found then it will return the mapping/target namespace
//
// velero doesn't pass the restore name to plugin, so we are following the below
// approach to fetch the namespace mapping:
//
// plugin find the relevant restore from the sorted list(creationTimestamp in decreasing order) of
// restore resource using following criteria:
//		- retore is in in-progress state AND
//		  backup for that restore matches with the backup name from snapshotID
// Above approach works because velero support sequential restore
func GetRestoreNamespace(ns, bkpName string, log logrus.FieldLogger) (string, error) {
	listOpts := metav1.ListOptions{}
	list, err := clientSet.VeleroV1().Restores(veleroNs).List(listOpts)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get list of restore")
	}

	sort.Sort(sort.Reverse(RestoreByCreationTimestamp(list.Items)))

	for _, r := range list.Items {
		if r.Status.Phase == velerov1api.RestorePhaseInProgress && r.Spec.BackupName == bkpName {
			targetedNs, ok := r.Spec.NamespaceMapping[ns]
			if ok {
				return targetedNs, nil
			}
			return ns, nil
		}
	}
	return "", errors.Errorf("restore not found for backup %s", bkpName)
}

// GetTargetNode return the node mapping for the given node
// if node mapping not found then it will return the same nodename in which backup was created
// if node mapping found then it will return the mapping/target nodename
func GetTargetNode(k8s *kubernetes.Clientset, node string) (string, error) {
	opts := metav1.ListOptions{
		LabelSelector: "velero.io/plugin-config,velero.io/change-pvc-node=RestoreItemAction",
	}

	list, err := k8s.CoreV1().ConfigMaps(veleroNs).List(opts)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get list of node mapping configmap")
	}

	if len(list.Items) == 0 {
		return node, nil
	}

	if len(list.Items) > 1 {
		var items []string
		for _, item := range list.Items {
			items = append(items, item.Name)
		}
		return "", errors.Errorf("found more than one ConfigMap matching label selector %q: %v", opts.LabelSelector, items)
	}

	config := list.Items[0]

	tnode, ok := config.Data[node]
	if !ok {
		return node, nil
	}

	return tnode, nil
}
