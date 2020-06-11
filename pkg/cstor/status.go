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
	"encoding/json"
	"time"

	"github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
)

// checkBackupStatus queries MayaAPI server for given backup status
// and wait until backup completes
func (p *Plugin) checkBackupStatus(bkp *v1alpha1.CStorBackup, isCSIVolume bool) {
	var (
		bkpDone bool
		url     string
	)

	if isCSIVolume {
		url = p.cvcAddr + backupEndpoint
	} else {
		url = p.mayaAddr + backupEndpoint
	}

	bkpvolume, exists := p.volumes[bkp.Spec.VolumeName]
	if !exists {
		p.Log.Errorf("Failed to fetch volume info for {%s}", bkp.Spec.VolumeName)
		p.cl.ExitServer = true
		bkpvolume.backupStatus = v1alpha1.BKPCStorStatusInvalid
		return
	}

	bkpData, err := json.Marshal(bkp)
	if err != nil {
		p.Log.Errorf("JSON marshal failed : %s", err.Error())
		p.cl.ExitServer = true
		bkpvolume.backupStatus = v1alpha1.BKPCStorStatusInvalid
		return
	}

	for !bkpDone {
		var bs v1alpha1.CStorBackup

		time.Sleep(backupStatusInterval * time.Second)
		resp, err := p.httpRestCall(url, "GET", bkpData)
		if err != nil {
			p.Log.Warnf("Failed to fetch backup status : %s", err.Error())
			continue
		}

		err = json.Unmarshal(resp, &bs)
		if err != nil {
			p.Log.Warnf("Unmarshal failed : %s", err.Error())
			continue
		}

		bkpvolume.backupStatus = bs.Status

		switch bs.Status {
		case v1alpha1.BKPCStorStatusDone, v1alpha1.BKPCStorStatusFailed, v1alpha1.BKPCStorStatusInvalid:
			bkpDone = true
			p.cl.ExitServer = true
			if err = p.cleanupCompletedBackup(bs, isCSIVolume); err != nil {
				p.Log.Warningf("failed to execute clean-up request for backup=%s err=%s", bs.Name, err)
			}
		}
	}
}

// checkRestoreStatus queries MayaAPI server for given restore status
// and wait until restore completes
func (p *Plugin) checkRestoreStatus(rst *v1alpha1.CStorRestore, vol *Volume) {
	var (
		rstDone bool
		url     string
	)

	if vol.isCSIVolume {
		url = p.cvcAddr + restorePath
	} else {
		url = p.mayaAddr + restorePath
	}

	rstData, err := json.Marshal(rst)
	if err != nil {
		p.Log.Errorf("JSON marshal failed : %s", err.Error())
		vol.restoreStatus = v1alpha1.RSTCStorStatusInvalid
		p.cl.ExitServer = true
	}

	for !rstDone {
		var rs v1alpha1.CStorRestore

		time.Sleep(restoreStatusInterval * time.Second)
		resp, err := p.httpRestCall(url, "GET", rstData)
		if err != nil {
			p.Log.Warnf("Failed to fetch backup status : %s", err.Error())
			continue
		}

		err = json.Unmarshal(resp, &rs.Status)
		if err != nil {
			p.Log.Warnf("Unmarshal failed : %s", err.Error())
			continue
		}

		vol.restoreStatus = rs.Status

		switch rs.Status {
		case v1alpha1.RSTCStorStatusDone, v1alpha1.RSTCStorStatusFailed, v1alpha1.RSTCStorStatusInvalid:
			rstDone = true
			p.cl.ExitServer = true
		}
	}
}

// cleanupCompletedBackup send the delete request to apiserver
// to cleanup backup resources
// If it is normal backup then it will delete the current backup, it can be failed or succeeded backup
// If it is scheduled backup then
//		- if current backup is base backup, not incremental one, then it will not perform any clean-up
//		- if current backup is incremental backup and failed one then it will delete that(current) backup
//		- if current backup is incremental backup and completed successfully then
//		  it will delete the last completed or previous backup
func (p *Plugin) cleanupCompletedBackup(bkp v1alpha1.CStorBackup, isCSIVolume bool) error {
	targetedSnapName := bkp.Spec.SnapName

	// In case of scheduled backup we are using the last completed backup to send
	// differential snapshot. So We don't need to delete the last completed backup.
	if isScheduledBackup(bkp) && isBackupSucceeded(bkp) {
		// For incremental backup We are using PrevSnapName to send the differential snapshot
		// Since Given backup is completed successfully We can delete the 2nd last completed backup
		if bkp.Spec.PrevSnapName == "" {
			// PrevSnapName will be empty if the given backup is base backup
			// clean-up is not required for base backup
			return nil
		}
		targetedSnapName = bkp.Spec.PrevSnapName
	}

	p.Log.Infof("executing clean-up request.. snapshot=%s volume=%s ns=%s backup=%s",
		targetedSnapName,
		bkp.Spec.VolumeName,
		bkp.Namespace,
		bkp.Spec.BackupName,
	)

	return p.sendDeleteRequest(targetedSnapName,
		bkp.Spec.VolumeName,
		bkp.Namespace,
		bkp.Spec.BackupName,
		isCSIVolume)
}

// return true if given backup is part of schedule
func isScheduledBackup(bkp v1alpha1.CStorBackup) bool {
	// if backup is scheduled backup then snapshot name and backup name are different
	return bkp.Spec.SnapName != bkp.Spec.BackupName
}

// isBackupSucceeded returns true if backup completed successfully
func isBackupSucceeded(bkp v1alpha1.CStorBackup) bool {
	return bkp.Status == v1alpha1.BKPCStorStatusDone
}
