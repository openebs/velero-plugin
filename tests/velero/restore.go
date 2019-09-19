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
	"sort"
	"time"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/pkg/errors"
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
	//return rc[i].CreationTimestamp.Before(&rc[j].CreationTimestamp)
}

func (c *ClientSet) generateRestoreName(backup string) (string, error) {
	for i := 0; i < 10; {
		r := generateRandomString(8) + "-" + backup
		if len(r) == 0 {
			continue
		}
		_, err := c.VeleroV1().
			Restores(VeleroNamespace).
			Get(r, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			return r, nil
		}
	}
	return "", errors.New("Failed to generate unique restore name")
}

// GetScheduledBackups list out the generated backup for given schedule
func (c *ClientSet) GetScheduledBackups(schedule string) ([]string, error) {
	var bkplist []string

	olist, err := c.VeleroV1().
		Backups(VeleroNamespace).
		List(metav1.ListOptions{
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

// CreateRestoreFromSchedule restore from given schedule's n'th backup for ns Namespace
func (c *ClientSet) CreateRestoreFromSchedule(ns, schedule string, n int) (v1.RestorePhase, error) {
	var status v1.RestorePhase
	var err error

	bkplist, err := c.GetScheduledBackups(schedule)
	if err != nil {
		return "", err
	}
	bkplist = bkplist[n:]
	for _, bkp := range bkplist {
		if status, err = c.CreateRestore(ns, bkp); status != v1.RestorePhaseCompleted {
			break
		}
	}
	return status, err
}

// CreateRestore create restore from given backup for ns Namespace
func (c *ClientSet) CreateRestore(ns, backup string) (v1.RestorePhase, error) {
	var status v1.RestorePhase
	snapVolume := true

	restoreName, err := c.generateRestoreName(backup)
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
		},
	}
	o, err := c.VeleroV1().
		Restores(VeleroNamespace).
		Create(rst)
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
		Get(name, metav1.GetOptions{})
}

// GetRestoredSnapshotFromSchedule list out the snapshot restored from given schedule
func (c *ClientSet) GetRestoredSnapshotFromSchedule(scheduleName string) (map[string]v1.RestorePhase, error) {
	snapshotList := make(map[string]v1.RestorePhase)

	bkplist, err := c.GetScheduledBackups(scheduleName)
	if err != nil {
		return snapshotList, err
	}

	restoreList, err := c.VeleroV1().
		Restores(VeleroNamespace).List(metav1.ListOptions{})
	if err == nil {
		for _, r := range restoreList.Items {
			for _, b := range bkplist {
				if r.Spec.BackupName == b && isRestoreDone(&r) {
					snapshotList[b] = r.Status.Phase
				}
			}
		}
	}

	return snapshotList, nil
}
