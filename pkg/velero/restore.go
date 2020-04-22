package velero

import (
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRestoreNamespace return the namespace mapping for the given namespace
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
