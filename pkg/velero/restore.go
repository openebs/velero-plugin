package velero

import (
	"context"
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
//   - retore is in in-progress state AND
//     backup for that restore matches with the backup name from snapshotID
//
// Above approach works because velero support sequential restore
func GetRestoreNamespace(ns, bkpName string, log logrus.FieldLogger) (string, error) {
	list := &velerov1api.RestoreList{}
	if err := kubeClient.List(context.TODO(), list, client.InNamespace(veleroNs)); err != nil {
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
		LabelSelector: "velero.io/plugin-config,velero.io/change-pvc-node-selector=RestoreItemAction",
	}

	list, err := k8s.CoreV1().ConfigMaps(veleroNs).List(context.TODO(), opts)
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
