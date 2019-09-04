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
	"github.com/pkg/errors"
)

// checkBackupStatus queries MayaAPI server for given backup status
// and wait until backup completes
func (p *Plugin) checkBackupStatus(bkp *v1alpha1.CStorBackup) {
	var bkpDone bool

	url := p.mayaAddr + backupEndpoint

	bkpvolume, exists := p.volumes[bkp.Spec.VolumeName]
	if !exists {
		p.Log.Errorf("Failed to fetch volume info for {%s}", bkp.Spec.VolumeName)
		panic(errors.Errorf("Failed to fetch volume info for {%s}", bkp.Spec.VolumeName))
	}

	bkpData, err := json.Marshal(bkp)
	if err != nil {
		p.Log.Errorf("JSON marshal failed : %s", err.Error())
		panic(errors.Errorf("JSON marshal failed : %s", err.Error()))
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
		}
	}
}

// checkRestoreStatus queries MayaAPI server for given restore status
// and wait until restore completes
func (p *Plugin) checkRestoreStatus(rst *v1alpha1.CStorRestore, vol *Volume) {
	var rstDone bool

	url := p.mayaAddr + restorePath

	rstData, err := json.Marshal(rst)
	if err != nil {
		p.Log.Errorf("JSON marshal failed : %s", err.Error())
		panic(errors.Errorf("JSON marshal failed : %s", err.Error()))
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
