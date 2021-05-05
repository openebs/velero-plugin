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
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/openebs/velero-plugin/pkg/zfs/utils"
	apis "github.com/openebs/zfs-localpv/pkg/apis/openebs.io/zfs/v1"
	"github.com/openebs/zfs-localpv/pkg/builder/bkpbuilder"
	"github.com/openebs/zfs-localpv/pkg/builder/volbuilder"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VeleroBkpKey  = "velero.io/backup"
	VeleroSchdKey = "velero.io/schedule-name"
	VeleroVolKey  = "velero.io/volname"
	VeleroNsKey   = "velero.io/namespace"
)

func (p *Plugin) getPV(volumeID string) (*v1.PersistentVolume, error) {
	return p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(context.TODO(), volumeID, metav1.GetOptions{})
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
	pvname, _, snapname, err := utils.GetInfoFromSnapshotID(snapshotID)
	if err != nil {
		return err
	}

	bkpname := utils.GenerateResourceName(pvname, snapname)
	err = bkpbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(bkpname)
	if err != nil {
		p.Log.Errorf("zfs: Failed to delete the backup %s", snapshotID)
	}

	return err
}

func (p *Plugin) getPrevSnap(volname, schdname string) (string, error) {
	if p.incremental < 1 || len(schdname) == 0 {
		// not an incremental backup, take the full backup
		return "", nil
	}

	listOptions := metav1.ListOptions{
		LabelSelector: VeleroSchdKey + "=" + schdname + "," + VeleroVolKey + "=" + volname,
	}

	bkpList, err := bkpbuilder.NewKubeclient().
		WithNamespace(p.namespace).List(listOptions)

	if err != nil {
		return "", err
	}

	var backups []string

	size := len(bkpList.Items)
	count := p.incremental + 1

	if uint64(size)%count == 0 {
		// have to start the next snapshot incremental group, take the full backup
		return "", nil
	}

	/*
	 * Backup names are in the form of <schdeule>-<yyyymmddhhmmss>
	 * to get the last snapshot, sort the list of successful backups,
	 * the previous snapshot will be the last element in the sorted list
	 */
	if len(bkpList.Items) > 0 {
		for _, bkp := range bkpList.Items {
			if bkp.Status == apis.BKPZFSStatusDone {
				backups = append(backups, bkp.Spec.SnapName)
			}
		}
		size := len(backups)
		sort.Strings(backups)
		return backups[size-1], nil
	}

	return "", nil
}

func (p *Plugin) createBackup(vol *apis.ZFSVolume, schdname, snapname string, port int) (string, error) {
	bkpname := utils.GenerateResourceName(vol.Name, snapname)

	p.Log.Debugf("zfs: creating ZFSBackup vol = %s bkp = %s schd = %s", vol.Name, bkpname, schdname)

	var err error
	labels := map[string]string{}
	prevSnap := ""

	if len(schdname) > 0 {
		// add schdeule name as label
		labels[VeleroSchdKey] = schdname
		labels[VeleroVolKey] = vol.Name
		prevSnap, err = p.getPrevSnap(vol.Name, schdname)
		if err != nil {
			p.Log.Errorf("zfs: Failed to get prev snapshot bkp %s err: {%v}", snapname, err)
			return "", err
		}
	}

	p.Log.Debugf("zfs: backup incr(%d) schd=%s snap=%s prevsnap=%s vol=%s", p.incremental, schdname, snapname, prevSnap, vol.Name)

	serverAddr := p.remoteAddr + ":" + strconv.Itoa(port)

	bkp, err := bkpbuilder.NewBuilder().
		WithName(bkpname).
		WithLabels(labels).
		WithVolume(vol.Name).
		WithPrevSnap(prevSnap).
		WithSnap(snapname).
		WithNode(vol.Spec.OwnerNodeID).
		WithStatus(apis.BKPZFSStatusInit).
		WithRemote(serverAddr).
		Build()

	if err != nil {
		return "", err
	}
	_, err = bkpbuilder.NewKubeclient().WithNamespace(p.namespace).Create(bkp)
	if err != nil {
		return "", err
	}

	return bkpname, nil
}

func (p *Plugin) checkBackupStatus(bkpname string) error {
	for {
		getOptions := metav1.GetOptions{}
		bkp, err := bkpbuilder.NewKubeclient().
			WithNamespace(p.namespace).Get(bkpname, getOptions)

		if err != nil {
			p.Log.Errorf("zfs: Failed to fetch backup info {%s}", bkpname)
			return errors.Errorf("zfs: error in getting bkp status err %v", err)
		}

		switch bkp.Status {
		case apis.BKPZFSStatusDone:
			return nil
		case apis.BKPZFSStatusFailed, apis.BKPZFSStatusInvalid:
			return errors.Errorf("zfs: error in uploading snapshot, status:{%v}", bkp.Status)
		}

		time.Sleep(backupStatusInterval * time.Second)
	}
}

func (p *Plugin) doUpload(wg *sync.WaitGroup, filename string, size int64, port int) {
	defer wg.Done()

	ok := p.cl.Upload(filename, size, port)
	if !ok {
		p.Log.Errorf("zfs: Failed to upload file %s", filename)
		*p.cl.ConnReady <- false
	}
	// done with the channel, close it
	close(*p.cl.ConnReady)
}

func (p *Plugin) doBackup(volumeID string, snapname string, schdname string, port int) (string, error) {
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

	filename := p.cl.GenerateRemoteFileWithSchd(volumeID, schdname, snapname)
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

	p.Log.Debugf("zfs: uploading Snapshot %s file %s", snapname, filename)

	// reset the connection state
	p.cl.ConnStateReset()

	var wg sync.WaitGroup

	wg.Add(1)
	go p.doUpload(&wg, filename, size, port)

	// wait for the upload server to exit
	defer func() {
		p.cl.ExitServer = true
		wg.Wait()
		p.cl.ConnReady = nil
	}()

	// wait for the connection to be ready
	ok := p.cl.ConnReadyWait()
	if !ok {
		return "", errors.New("zfs: error in uploading snapshot")
	}

	bkpname, err := p.createBackup(vol, schdname, snapname, port)
	if err != nil {
		return "", err
	}

	err = p.checkBackupStatus(bkpname)
	if err != nil {
		p.deleteBackup(bkpname)
		p.Log.Errorf("zfs: backup failed vol %s snap %s bkpname %s err: %v", volumeID, snapname, bkpname, err)
		return "", err
	}

	// generate the snapID
	snapID := utils.GenerateSnapshotID(volumeID, schdname, snapname)

	p.Log.Debugf("zfs: backup done vol %s bkp %s snapID %s", volumeID, bkpname, snapID)

	return snapID, nil
}
