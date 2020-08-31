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

package plugin

import (
	"time"
	"sort"
	"encoding/json"
	"strconv"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"github.com/openebs/velero-plugin/pkg/zfs/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/openebs/zfs-localpv/pkg/builder/volbuilder"
	"github.com/openebs/zfs-localpv/pkg/builder/bkpbuilder"
	apis "github.com/openebs/zfs-localpv/pkg/apis/openebs.io/zfs/v1"
)

const (
	VeleroBkpKey = "velero.io/backup"
	VeleroSchdKey = "velero.io/schedule-name"
	VeleroVolKey = "velero.io/volname"
	VeleroNsKey = "velero.io/namespace"
)


func (p *Plugin) getPV(volumeID string) (*v1.PersistentVolume, error) {
	return p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(volumeID, metav1.GetOptions{})
}

func (p *Plugin) uploadZFSVolume(vol *apis.ZFSVolume, filename string) error {
	data, err := json.MarshalIndent(vol, "", "\t")
	if err != nil {
		return errors.New("zfs: error doing json parsing")
	}

	if ok := p.cl.Write(data, filename+".zfsvol"); !ok {
		return errors.New("zfs: failed to upload ZFSVolume")
	}

	return nil
}

// deleteBackup deletes the backup resource
func (p *Plugin) deleteBackup(snapshotID string) error {
	err := bkpbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(snapshotID)
	if err != nil {
		p.Log.Errorf("zfs: Failed to delete the backup %s", snapshotID)
	}

	return err
}

func (p *Plugin) getPrevSnap(volname, schdname string) (string, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: VeleroSchdKey + "=" + schdname + "," + VeleroVolKey + "=" + volname,
	}

	bkpList, err := bkpbuilder.NewKubeclient().
		WithNamespace(p.namespace).List(listOptions)

	if err != nil {
		return "", err
	}

	var backups []string

	/*
	 * Backup names are in the form of <schdeule>-<yyyymmddhhmmss>
	 * to get the last snapshot, sort the list of successful backups,
	 * the previous snapshot will be the last element in the sorted list
	 */
	if len (bkpList.Items) > 0 {
		for _, bkp := range bkpList.Items {
			if bkp.Status == apis.BKPZFSStatusDone {
				backups = append(backups, bkp.Spec.SnapName)
			}
		}
		size := len(backups)
		sort.Strings(backups)
		return backups[size - 1], nil
	}

	return "", nil
}

func (p *Plugin) createBackup(vol *apis.ZFSVolume, schdname, snapname string) (string, error) {
	bkpname := utils.GenerateSnapshotID(vol.Name, snapname)

	p.Log.Debugf("zfs: creating ZFSBackup vol = %s bkp = %s schd = %s", vol.Name, bkpname, schdname)

	var err error
	labels := map[string]string{}
	prevSnap := ""

	if len(schdname) > 0 {
		// add schdeule name as label
		labels[VeleroSchdKey] = schdname
		labels[VeleroVolKey] = vol.Name
		if p.incremental {
			prevSnap, err = p.getPrevSnap(vol.Name, schdname)
			if err != nil {
				p.Log.Errorf("zfs: Failed to get prev snapshot bkp %s err: {%v}", snapname, err)
				return "", err
			}
		}
	}

	bkp, err := bkpbuilder.NewBuilder().
		WithName(bkpname).
		WithLabels(labels).
		WithVolume(vol.Name).
		WithPrevSnap(prevSnap).
		WithSnap(snapname).
		WithNode(vol.Spec.OwnerNodeID).
		WithStatus(apis.BKPZFSStatusInit).
		WithRemote(p.remoteAddr).
		Build()

	if err != nil {
		return "", err
	}
	_, err = bkpbuilder.NewKubeclient().WithNamespace(p.namespace).Create(bkp)
	if err != nil {
		return "", err
	}

	return bkpname, err
}

func (p *Plugin) checkBackupStatus(bkpname string) {
	bkpDone := false

	for !bkpDone {
		getOptions := metav1.GetOptions{}
	        bkp, err := bkpbuilder.NewKubeclient().
		        WithNamespace(p.namespace).Get(bkpname, getOptions)

		if err != nil {
			p.Log.Errorf("zfs: Failed to fetch backup info {%s}", bkpname)
			p.cl.ExitServer = true
			return
		}

		time.Sleep(backupStatusInterval * time.Second)

		switch bkp.Status {
		case apis.BKPZFSStatusDone, apis.BKPZFSStatusFailed, apis.BKPZFSStatusInvalid:
			bkpDone = true
			p.cl.ExitServer = true
		}
	}
}

func (p *Plugin) doBackup(volumeID string, snapname string, schdname string) (string, error) {
	pv, err := p.getPV(volumeID)
	if err != nil {
		p.Log.Errorf("zfs: Failed to get pv %s snap %s schd %s err %v", volumeID, snapname, schdname, err)
		return "", err
	}

	if pv.Spec.PersistentVolumeSource.CSI == nil {
		return "", errors.New("zfs: err not a CSI pv")
	}

	volHandle := pv.Spec.PersistentVolumeSource.CSI.VolumeHandle

        getOptions := metav1.GetOptions{}
        vol, err := volbuilder.NewKubeclient().
                WithNamespace(p.namespace).Get(volHandle, getOptions)
	if err != nil {
		return "", err
	}

	if pv.Spec.ClaimRef != nil {
		// add source namespace in the label to filter it at restore time
		if vol.Labels == nil {
			vol.Labels = map[string]string{}
		}
		vol.Labels[VeleroNsKey] = pv.Spec.ClaimRef.Namespace
	} else {
		return "", errors.Errorf("zfs: err pv is not claimed")
	}

	// reset the exit server to false
	p.cl.ExitServer = false

	filename := p.cl.GenerateRemoteFilename(volumeID, snapname)
	if filename == "" {
		return "", errors.Errorf("zfs: error creating remote file name for backup")
	}

	err = p.uploadZFSVolume(vol, filename)
	if err != nil {
		return "", err
	}

	size, err := strconv.ParseInt(vol.Spec.Capacity, 10, 64)
	if err != nil {
		return "", errors.Errorf("zfs: error parsing the size %s", vol.Spec.Capacity)
	}

	// TODO(pawan) should wait for upload server to be up
	bkpname, err := p.createBackup(vol, schdname, snapname)
	if err != nil {
		return "", err
	}

	go p.checkBackupStatus(bkpname)

	p.Log.Debugf("zfs: uploading Snapshot %s file %s", snapname, filename)
	ok := p.cl.Upload(filename, size)
	if !ok {
		p.deleteBackup(bkpname)
		return "", errors.New("zfs: error in uploading snapshot")
	}

        bkp, err := bkpbuilder.NewKubeclient().
	        WithNamespace(p.namespace).Get(bkpname, metav1.GetOptions{})

	if err != nil {
		p.deleteBackup(bkpname)
		return "", err
	}

	if bkp.Status != apis.BKPZFSStatusDone {
		p.deleteBackup(bkpname)
		return "", errors.Errorf("zfs: error in uploading snapshot, status:{%v}", bkp.Status)
	}

	return bkpname, nil
}
