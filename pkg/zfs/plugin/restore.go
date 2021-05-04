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
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/openebs/velero-plugin/pkg/velero"
	"github.com/openebs/velero-plugin/pkg/zfs/utils"
	apis "github.com/openebs/zfs-localpv/pkg/apis/openebs.io/zfs/v1"
	"github.com/openebs/zfs-localpv/pkg/builder/restorebuilder"
	"github.com/openebs/zfs-localpv/pkg/builder/volbuilder"
	"github.com/openebs/zfs-localpv/pkg/zfs"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	restoreStatusInterval = 5
)

func (p *Plugin) buildZFSVolume(pvname string, bkpname string, bkpZV *apis.ZFSVolume) (*apis.ZFSVolume, error) {
	// get the target namespace
	ns, err := velero.GetRestoreNamespace(bkpZV.Labels[VeleroNsKey], bkpname, p.Log)

	if err != nil {
		p.Log.Errorf("zfs: failed to get target ns for pv=%s, bkpname=%s err: %v", pvname, bkpname, err)
		return nil, err
	}

	filter := metav1.ListOptions{
		LabelSelector: VeleroVolKey + "=" + pvname + "," + VeleroNsKey + "=" + ns,
	}
	volList, err := volbuilder.NewKubeclient().WithNamespace(p.namespace).List(filter)

	if err != nil {
		p.Log.Errorf("zfs: failed to get source volume failed vol %s snap %s err: %v", pvname, bkpname, err)
		return nil, err
	}

	if len(volList.Items) > 0 {
		return nil, errors.Errorf("zfs: err pv %s has already been restored bkpname %s", pvname, bkpname)
	}

	// this is first full restore, go ahead and create the volume
	rZV := &apis.ZFSVolume{}
	// hack(https://github.com/vmware-tanzu/velero/pull/2835): generate a new uuid only if PV exist
	pv, err := p.getPV(pvname)

	if err == nil && pv != nil {
		rvol, err := utils.GetRestorePVName()
		if err != nil {
			return nil, errors.Errorf("zfs: failed to get restore vol name for %s", pvname)
		}
		rZV.Name = rvol
	} else {
		rZV.Name = pvname
	}

	rZV.Spec = bkpZV.Spec

	// if restored volume was a clone, create a new volume instead of cloning it from a snaphsot
	rZV.Spec.SnapName = ""

	// get the target node
	tnode, err := velero.GetTargetNode(p.K8sClient, rZV.Spec.OwnerNodeID)
	if err != nil {
		return nil, err
	}

	// update the target node name
	p.Log.Debugf("zfs: GetTargetNode node %s=>%s", rZV.Spec.OwnerNodeID, tnode)
	rZV.Spec.OwnerNodeID = tnode

	// set the volume status as pending
	rZV.Status.State = zfs.ZFSStatusPending

	// add original volume and schedule name in the label
	rZV.Labels = map[string]string{VeleroVolKey: pvname, VeleroNsKey: ns}
	rZV.Annotations = map[string]string{VeleroBkpKey: bkpname}

	return rZV, nil
}

func (p *Plugin) createZFSVolume(rZV *apis.ZFSVolume) error {

	_, err := volbuilder.NewKubeclient().WithNamespace(p.namespace).Create(rZV)
	if err != nil {
		p.Log.Errorf("zfs: create ZFSVolume failed vol %v err: %v", rZV, err)
		return err
	}

	err = p.checkVolCreation(rZV.Name)
	if err != nil {
		p.Log.Errorf("zfs: checkVolCreation failed %s err: %v", rZV.Name, err)
		return err
	}

	return nil
}

func (p *Plugin) getZFSVolume(pvname, schdname, bkpname string) (*apis.ZFSVolume, error) {
	bkpZV := &apis.ZFSVolume{}

	filename := p.cl.GenerateRemoteFileWithSchd(pvname, schdname, bkpname)

	data, ok := p.cl.Read(filename + ".zfsvol")
	if !ok {
		return nil, errors.Errorf("zfs: failed to download ZFSVolume file=%s", filename+".zfsvol")
	}

	if err := json.Unmarshal(data, bkpZV); err != nil {
		return nil, errors.Errorf("zfs: failed to decode zfsvolume file=%s", filename+".zfsvol")
	}

	return p.buildZFSVolume(pvname, bkpname, bkpZV)
}

func (p *Plugin) isVolumeReady(volumeID string) (ready bool, err error) {
	getOptions := metav1.GetOptions{}
	vol, err := volbuilder.NewKubeclient().
		WithNamespace(p.namespace).Get(volumeID, getOptions)

	if err != nil {
		return false, err
	}

	return vol.Status.State == zfs.ZFSStatusReady, nil
}

func (p *Plugin) checkRestoreStatus(rname string) error {
	defer func() {
		err := restorebuilder.NewKubeclient().WithNamespace(p.namespace).Delete(rname)
		if err != nil {
			// ignore error
			p.Log.Errorf("zfs: delete restore %s failed err: %v", rname, err)
		}
	}()

	for {
		getOptions := metav1.GetOptions{}
		rstr, err := restorebuilder.NewKubeclient().
			WithNamespace(p.namespace).Get(rname, getOptions)

		if err != nil {
			p.Log.Errorf("zfs: Failed to fetch restore {%s}", rname)
			return errors.Errorf("zfs: error in getting restore status %s err %v", rname, err)
		}

		switch rstr.Status {
		case apis.RSTZFSStatusDone:
			return nil
		case apis.RSTZFSStatusFailed, apis.RSTZFSStatusInvalid:
			return errors.Errorf("zfs: error in restoring %s, status:{%v}", rname, rstr.Status)
		}

		time.Sleep(restoreStatusInterval * time.Second)
	}
}

func (p *Plugin) checkVolCreation(volname string) error {

	for true {

		getOptions := metav1.GetOptions{}
		vol, err := volbuilder.NewKubeclient().
			WithNamespace(p.namespace).Get(volname, getOptions)

		if err != nil {
			p.Log.Errorf("zfs: Failed to fetch volume {%s}", volname)
			return err
		}

		switch vol.Status.State {
		case zfs.ZFSStatusReady:
			return nil
		case zfs.ZFSStatusFailed:
			return errors.Errorf("zfs: Error creating remote file name for restore")
		}
		time.Sleep(restoreStatusInterval * time.Second)
	}
	return nil
}

// startRestore creates the ZFSRestore CR to start downloading the data and returns ZFSRestore CR name
func (p *Plugin) startRestore(zv *apis.ZFSVolume, bkpname string, port int) (string, error) {
	node := zv.Spec.OwnerNodeID
	serverAddr := p.remoteAddr + ":" + strconv.Itoa(port)
	zfsvol := zv.Name
	rname := utils.GenerateResourceName(zfsvol, bkpname)

	rstr, err := restorebuilder.NewBuilder().
		WithName(rname).
		WithVolume(zfsvol).
		WithNode(node).
		WithStatus(apis.RSTZFSStatusInit).
		WithRemote(serverAddr).
		WithVolSpec(zv.Spec).
		Build()

	if err != nil {
		return "", err
	}

	_, err = restorebuilder.NewKubeclient().WithNamespace(p.namespace).Create(rstr)

	if err != nil {
		return "", err
	}
	return rname, nil
}

func (p *Plugin) doDownload(wg *sync.WaitGroup, filename string, port int) {
	defer wg.Done()

	ok := p.cl.Download(filename, port)
	if !ok {
		p.Log.Errorf("zfs: failed to download the file %s", filename)
		*p.cl.ConnReady <- false
	}
	// done with the channel, close it
	close(*p.cl.ConnReady)
}

func (p *Plugin) dataRestore(zv *apis.ZFSVolume, pvname, schdname, bkpname string, port int) error {
	filename := p.cl.GenerateRemoteFileWithSchd(pvname, schdname, bkpname)
	if filename == "" {
		return errors.Errorf("zfs: Error creating remote file name for restore")
	}

	// reset the connection state
	p.cl.ConnStateReset()

	var wg sync.WaitGroup

	wg.Add(1)
	go p.doDownload(&wg, filename, port)

	// wait for the download server to exit
	defer func() {
		p.cl.ExitServer = true
		wg.Wait()
		p.cl.ConnReady = nil
	}()

	// wait for the connection to be ready
	ok := p.cl.ConnReadyWait()
	if !ok {
		return errors.Errorf("zfs: restore server is not ready")
	}

	rname, err := p.startRestore(zv, bkpname, port)
	if err != nil {
		p.Log.Errorf("zfs: restoreVolume failed vol %s snap %s err: %v", pvname, bkpname, err)
		return err
	}

	err = p.checkRestoreStatus(rname)
	if err != nil {
		p.Log.Errorf("zfs: restore failed vol %s snap %s err: %v", pvname, bkpname, err)
		return err
	}

	p.Log.Debugf("zfs: restore done vol %s => %s bkp %s", pvname, zv.Name, bkpname)
	return nil
}

func (p *Plugin) getSnapList(pvname, schdname, bkpname string) ([]string, error) {
	list := []string{bkpname}

	if p.incremental < 1 || len(schdname) == 0 {
		// not an incremental backup, return the list having bkpname
		return list, nil
	}

	filename := p.cl.GetFileNameWithSchd(pvname, schdname)

	snapList, err := p.cl.GetSnapListFromCloud(filename, schdname)
	if err != nil {
		return list, err
	}

	sort.Strings(snapList)

	var size uint64 = 0

	// get the index of the backup
	for idx, snap := range snapList {
		if snap == bkpname {
			size = uint64(idx) + 1
			break
		}
	}

	if size == 0 {
		return list, errors.Errorf("zfs: error backup not found in snap list %s", bkpname)
	}

	// add the full backup count to get the closest full snapshot index for the backup
	count := p.incremental + 1

	// get the index of full backup
	fullBkpIdx := (uint64(size-1) / count) * count
	list = snapList[fullBkpIdx:size]

	return list, nil
}

func (p *Plugin) doRestore(snapshotID string, port int) (string, error) {
	pvname, schdname, bkpname, err := utils.GetInfoFromSnapshotID(snapshotID)
	if err != nil {
		return "", err
	}

	bkpList, err := p.getSnapList(pvname, schdname, bkpname)
	if err != nil {
		return "", err
	}

	if len(bkpList) == 0 {
		return "", errors.Errorf("zfs: error empty restore list %s", bkpname)
	}

	p.Log.Debugf("zfs: backup list for restore %v", bkpList)

	zv, err := p.getZFSVolume(pvname, schdname, bkpname)
	if err != nil {
		p.Log.Errorf("zfs: restore ZFSVolume failed vol %s bkp %s err %v", pvname, bkpname, err)
		return "", err
	}

	// attempt the incremental restore, will resote single backup if it is not a incremental backup
	for _, bkp := range bkpList {
		err = p.dataRestore(zv, pvname, schdname, bkp, port)

		if err != nil {
			p.Log.Errorf("zfs: error doRestore returning snap %s err %v", snapshotID, err)
			return "", err
		}
	}

	// restore done, create the ZFSVolume
	err = p.createZFSVolume(zv)
	if err != nil {
		p.Log.Errorf("zfs: can not create ZFS Volume, snap %s err %v", snapshotID, err)
		return "", err
	}

	return zv.Name, nil
}
