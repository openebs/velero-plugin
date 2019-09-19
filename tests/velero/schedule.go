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

package sanity

import (
	"fmt"
	"time"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *ClientSet) generateScheduleName() (string, error) {
	for i := 0; i < 10; {
		b := generateRandomString(8)
		if len(b) == 0 {
			continue
		}
		_, err := c.VeleroV1().
			Schedules(VeleroNamespace).
			Get(b, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			return b, nil
		}
	}
	return "", errors.New("Failed to generate unique backup name")
}

// CreateSchedule create scheduled backup for given namespace and wait till 'count' backup completed
func (c *ClientSet) CreateSchedule(ns, period string, count int) (string, v1.BackupPhase, error) {
	var status v1.BackupPhase
	snapVolume := true

	sname, err := c.generateScheduleName()
	if err != nil {
		return "", status, err
	}

	sched := &v1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sname,
			Namespace: VeleroNamespace,
		},
		Spec: v1.ScheduleSpec{
			Template: v1.BackupSpec{
				IncludedNamespaces:      []string{ns},
				SnapshotVolumes:         &snapVolume,
				StorageLocation:         BackupLocation,
				VolumeSnapshotLocations: []string{SnapshotLocation},
			},
			Schedule: period,
		},
	}
	o, err := c.VeleroV1().
		Schedules(VeleroNamespace).
		Create(sched)
	if err != nil {
		return "", status, err
	}

	if count < 0 {
		return o.Name, status, nil
	}
	if status, err = c.waitForScheduleCompletion(o.Name, count); err == nil {
		return o.Name, status, nil
	}
	return o.Name, status, err
}

func (c *ClientSet) waitForScheduleCompletion(name string, count int) (v1.BackupPhase, error) {
	for bcount := 0; bcount < count; {
		olist, err := c.VeleroV1().
			Backups(VeleroNamespace).
			List(metav1.ListOptions{
				LabelSelector: v1.ScheduleNameLabel + "=" + name,
			})
		if err != nil {
			return "", err
		}
		bcount = 0
		for _, bkp := range olist.Items {
			bkp := bkp
			if isBackupDone(&bkp) {
				if bkp.Status.Phase != v1.BackupPhaseCompleted {
					return bkp.Status.Phase,
						errors.Errorf("Backup{%s} failed.. %s", bkp.Name, bkp.Status.Phase)
				}
				bcount++
			}
		}
		fmt.Printf("Waiting for schedule %s completion completed Backup:%d/%d\n", name, bcount, count)
		time.Sleep(5 * time.Second)
	}
	return v1.BackupPhaseCompleted, nil
}

// DeleteSchedule delete given schedule
func (c *ClientSet) DeleteSchedule(schedule string) error {
	return c.VeleroV1().
		Schedules(VeleroNamespace).
		Delete(schedule, &metav1.DeleteOptions{})
}
