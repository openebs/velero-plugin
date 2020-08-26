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
	"encoding/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/openebs/zfs-localpv/pkg/zfs"
	"github.com/openebs/velero-plugin/pkg/velero"
	"github.com/pkg/errors"
	"github.com/openebs/velero-plugin/pkg/zfs/utils"
	"github.com/openebs/zfs-localpv/pkg/builder/volbuilder"
	"github.com/openebs/zfs-localpv/pkg/builder/restorebuilder"
	apis "github.com/openebs/zfs-localpv/pkg/apis/openebs.io/zfs/v1"
)

const (
	restoreStatusInterval = 5
)

func (p *Plugin) createVolume(pvname string, bkpname string, bkpZV *apis.ZFSVolume) (*apis.ZFSVolume, bool, error) {

	// get the target namespace
	ns, err := velero.GetRestoreNamespace(bkpZV.Labels[VeleroNsKey], bkpname, p.Log)

	if err != nil {
		p.Log.Errorf("zfs: failed to get target ns for pv=%s, bkpname=%s err: %v", pvname, bkpname, err)
		return nil, false, err
	}

	filter := metav1.ListOptions{
		LabelSelector: VeleroVolKey + "=" + pvname + "," + VeleroNsKey + "=" + ns,
	}
	volList, err := volbuilder.NewKubeclient().WithNamespace(p.namespace).List(filter)

	if err != nil {
		p.Log.Errorf("zfs: failed to get source volume failed vol %s snap %s err: %v", pvname, bkpname, err)
		return nil, false, err
	}

	var vol *apis.ZFSVolume = nil

	if len (volList.Items) > 1 {
		return nil, false, errors.Errorf("zfs: error can not have more than one source volume %s", pvname, bkpname)
	} else if len (volList.Items) == 1 {
		vol = &volList.Items[0]
		if !p.incremental ||
			bkpname == vol.Annotations[VeleroBkpKey] {
			// volume has already been restored
			return vol, false, errors.Errorf("zfs: pv %s is already restored bkpname %s", pvname, bkpname)
		}

		p.Log.Debugf("zfs: got existing volume %s for restore vol %s snap %s",  vol.Name, pvname, bkpname)
	}

	if vol == nil {
		// this is first full restore, go ahead and create the volume
		rZV := &apis.ZFSVolume{}
		// hack(https://github.com/vmware-tanzu/velero/pull/2835): generate a new uuid only if PV exist
		pv, err := p.getPV(pvname)

		if err == nil && pv != nil {
			rvol, err := utils.GetRestorePVName()
			if err != nil {
				return nil, false, errors.Errorf("zfs: failed to get restore vol name for %s", pvname)
			}
			rZV.Name = rvol
		} else {
			rZV.Name = pvname
		}

		rZV.Spec = bkpZV.Spec

		// if restored volume was a clone, create a new volume instead of cloning it from a snaphsot
		rZV.Spec.SnapName = ""

		// set the volume status as pending
		rZV.Status.State = zfs.ZFSStatusPending

		// add original volume and schedule name in the label
		rZV.Labels = map[string]string{VeleroVolKey : pvname, VeleroNsKey : ns}
		rZV.Annotations = map[string]string{VeleroBkpKey : bkpname}

		vol, err = volbuilder.NewKubeclient().WithNamespace(p.namespace).Create(rZV)
		if err != nil {
			p.Log.Errorf("zfs: create ZFSVolume failed vol %v err: %v", rZV, err)
			return nil, false, err
		}

		err = p.checkVolCreation(rZV.Name)
		if err != nil {
			p.Log.Errorf("zfs: checkVolCreation failed %s err: %v", rZV.Name, err)
			return nil, false, err
		}
	} else {
		// this is incremental restore, update the ZFS volume
		vol.Spec = bkpZV.Spec
		vol, err := volbuilder.NewKubeclient().WithNamespace(p.namespace).Update(vol)
		if err != nil {
			p.Log.Errorf("zfs: update ZFSVolume failed vol %v err: %v", vol, err)
			return nil, false, err
		}
	}

	return vol, true, nil
}

func (p *Plugin) restoreZFSVolume(pvname, bkpname string) (*apis.ZFSVolume, bool, error) {

	bkpZV := &apis.ZFSVolume{}

	filename := p.cl.GenerateRemoteFilename(pvname, bkpname)

	data, ok := p.cl.Read(filename + ".zfsvol")
	if !ok {
		return nil, false,  errors.Errorf("zfs: failed to download ZFSVolume file=%s", filename+".zfsvol")
	}

	if err := json.Unmarshal(data, bkpZV); err != nil {
		return nil, false, errors.Errorf("zfs: failed to decode zfsvolume file=%s", filename+".zfsvol")
	}

	return p.createVolume(pvname, bkpname, bkpZV)
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

func (p *Plugin) checkRestoreStatus(snapname string) {

	for true {
		getOptions := metav1.GetOptions{}
	        rstr, err := restorebuilder.NewKubeclient().
		        WithNamespace(p.namespace).Get(snapname, getOptions)

		if err != nil {
			p.Log.Errorf("zfs: Failed to fetch restore {%s}", snapname)
			p.cl.ExitServer = true
			return
		}

		switch rstr.Status {
		case apis.RSTZFSStatusDone, apis.RSTZFSStatusFailed, apis.RSTZFSStatusInvalid:
			p.cl.ExitServer = true
			return
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

func (p *Plugin) cleanupRestore(oldvol, newvol, rname string) error {
        rstr, err := restorebuilder.NewKubeclient().
	        WithNamespace(p.namespace).Get(rname, metav1.GetOptions{})

	if err != nil {
		p.Log.Errorf("zfs: get restore failed vol %s => %s snap %s err: %v", oldvol, newvol, rname, err)
		return err
	}

	err = restorebuilder.NewKubeclient().WithNamespace(p.namespace).Delete(rname)
	if err != nil {
		// ignore error
		p.Log.Errorf("zfs: delete restore failed vol %s => %s snap %s err: %v", oldvol, newvol, rname, err)
	}

	if rstr.Status != apis.RSTZFSStatusDone {
		// delete the volume
		err = volbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(newvol)
		if err != nil {
			// ignore error
			p.Log.Errorf("zfs: delete vol failed vol %s => %s snap %s err: %v", oldvol, newvol, rname, err)
		}

		p.Log.Errorf("zfs: restoreVolume status failed vol %s => %s snap %s",  oldvol, newvol, rname)
		return errors.Errorf("zfs: Failed to restore snapshoti %s, status:{%v}", rname, rstr.Status)
	}

	return nil
}

// restoreVolume returns restored vol name and a boolean value indication if we need
// to restore the volume. If Volume is already restored, we don't need to restore it.
func (p *Plugin) restoreVolume(rname, volname, bkpname string) (string, bool, error) {
	zv, needRestore, err := p.restoreZFSVolume(volname, bkpname)
	if err != nil {
		p.Log.Errorf("zfs: restore ZFSVolume failed vol %s bkp %s err %v", volname, bkpname, err)
		return "", false, err
	}

	if needRestore == false {
		return zv.Name, false, nil
	}

	node := zv.Spec.OwnerNodeID

	rstr, err := restorebuilder.NewBuilder().
		WithName(rname).
		WithVolume(zv.Name).
		WithNode(node).
		WithStatus(apis.RSTZFSStatusInit).
		WithRemote(p.remoteAddr).
		Build()

	if err != nil {
		return "", false, err
	}
	_, err = restorebuilder.NewKubeclient().WithNamespace(p.namespace).Create(rstr)
	return zv.Name, true, err
}

func (p *Plugin) doRestore(snapshotID string) (string, error) {

	volname, bkpname, err := utils.GetInfoFromSnapshotID(snapshotID)
	if err != nil {
		return "", err
	}

	filename := p.cl.GenerateRemoteFilename(volname, bkpname)
	if filename == "" {
		return "", errors.Errorf("zfs: Error creating remote file name for restore")
	}

	newvol, needRestore, err := p.restoreVolume(snapshotID, volname, bkpname)
	if err != nil {
		p.Log.Errorf("zfs: restoreVolume failed vol %s snap %s err: %v", volname, bkpname, err)
		return "", err
	}

	if needRestore == false {
		// volume has already been restored
		p.Log.Debugf("zfs: pv already restored vol %s => %s snap %s", volname, newvol, snapshotID)
		return newvol, nil
	}

	go p.checkRestoreStatus(snapshotID)

	ret := p.cl.Download(filename)
	if !ret {
		p.cleanupRestore(volname, newvol, snapshotID)
		return "", errors.New("zfs: failed to restore snapshot")
	}

	if err := p.cleanupRestore(volname, newvol, snapshotID); err != nil {
		return "", err
	}

	p.Log.Debugf("zfs: restore done vol %s => %s snap %s", volname, newvol, snapshotID)
	return newvol, nil
}
