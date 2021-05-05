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
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type byCreationTimeStamp []v1.Backup

func (rc byCreationTimeStamp) Len() int {
	return len(rc)
}

func (rc byCreationTimeStamp) Swap(i, j int) {
	rc[i], rc[j] = rc[j], rc[i]
}

func (rc byCreationTimeStamp) Less(i, j int) bool {
	return rc[i].Name < rc[j].Name
}

func (c *ClientSet) generateRestoreName(backup string) (string, error) {
	for i := 0; i < 10; i++ {
		r := generateRandomString(8) + "-" + backup
		if r == "" {
			continue
		}
		_, err := c.VeleroV1().
			Restores(VeleroNamespace).
			Get(context.TODO(), r, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			return r, nil
		}
	}
	return "", errors.New("failed to generate unique restore name")
}

// GetScheduledBackups list out the generated backup for given schedule
func (c *ClientSet) GetScheduledBackups(schedule string) ([]string, error) {
	var bkplist []string

	olist, err := c.VeleroV1().
		Backups(VeleroNamespace).
		List(context.TODO(), metav1.ListOptions{
			LabelSelector: v1.ScheduleNameLabel + "=" + schedule,
		})
	if err != nil && k8serrors.IsNotFound(err) {
		return nil, err
	}

	sort.Sort(byCreationTimeStamp(olist.Items))

	for _, o := range olist.Items {
		bkplist = append(bkplist, o.Name)
	}
	return bkplist, nil
}

// CreateRestore create restore from given backup/schedule for ns Namespace to targetedNs
// Arguments:
// - ns : source namespace or namespace from backup
// - targetedNs : targeted namespace for namespace 'ns'
// - backup : name of the backup, from which restore will happen
// - schedule : name of schedule, from which restore should happen. If mentioned, backup should be empty
func (c *ClientSet) CreateRestore(ns, targetedNs, backup, schedule string) (v1.RestorePhase, error) {
	var (
		status      v1.RestorePhase
		restoreName string
		err         error
	)

	snapVolume := true
	nsMapping := make(map[string]string)

	if targetedNs != "" && ns != targetedNs {
		nsMapping[ns] = targetedNs
	}

	if backup != "" {
		restoreName, err = c.generateRestoreName(backup)
	} else {
		restoreName, err = c.generateRestoreName(schedule)
	}

	if err != nil {
		return status, err
	}

	rst := &v1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: VeleroNamespace,
		},
		Spec: v1.RestoreSpec{
			IncludedNamespaces: []string{ns},
			RestorePVs:         &snapVolume,
			BackupName:         backup,
			ScheduleName:       schedule,
			NamespaceMapping:   nsMapping,
		},
	}
	o, err := c.VeleroV1().
		Restores(VeleroNamespace).
		Create(context.TODO(), rst, metav1.CreateOptions{})
	if err != nil {
		return status, err
	}

	return c.waitForRestoreCompletion(o.Name)
}

func (c *ClientSet) waitForRestoreCompletion(rst string) (v1.RestorePhase, error) {
	dumpLog := 0
	for {
		rst, err := c.getRestore(rst)
		if err != nil {
			return "", err
		}
		if isRestoreDone(rst) {
			return rst.Status.Phase, nil
		}
		if dumpLog > 6 {
			fmt.Printf("Waiting for restore %s completion..\n", rst.Name)
			dumpLog = 0
		}
		dumpLog++
		time.Sleep(5 * time.Second)
	}
}

func (c *ClientSet) getRestore(name string) (*v1.Restore, error) {
	return c.VeleroV1().
		Restores(VeleroNamespace).
		Get(context.TODO(), name, metav1.GetOptions{})
}

// GetRestoredSnapshotFromSchedule list out the snapshot restored from given schedule
func (c *ClientSet) GetRestoredSnapshotFromSchedule(scheduleName string) (map[string]v1.RestorePhase, error) {
	snapshotList := make(map[string]v1.RestorePhase)

	bkplist, err := c.GetScheduledBackups(scheduleName)
	if err != nil {
		return snapshotList, err
	}

	restoreList, err := c.VeleroV1().
		Restores(VeleroNamespace).List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, r := range restoreList.Items {
			restore := r
			for _, b := range bkplist {
				if r.Spec.BackupName == b && isRestoreDone(&restore) {
					snapshotList[b] = r.Status.Phase
				}
			}
		}
	}

	return snapshotList, nil
}
