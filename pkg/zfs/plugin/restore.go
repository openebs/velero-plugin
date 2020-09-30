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

func (p *Plugin) createVolume(pvname string, bkpname string, bkpZV *apis.ZFSVolume) (*apis.ZFSVolume, error) {
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

	var vol *apis.ZFSVolume = nil

	if len(volList.Items) > 1 {
		return nil, errors.Errorf("zfs: error can not have more than one source volume %s bkpname %s", pvname, bkpname)
	} else if len(volList.Items) == 1 {
		vol = &volList.Items[0]
		if !p.incremental ||
			bkpname == vol.Annotations[VeleroBkpKey] {
			// volume has already been restored
			return vol, errors.Errorf("zfs: pv %s is already restored bkpname %s", pvname, bkpname)
		}

		p.Log.Debugf("zfs: got existing volume %s for restore vol %s snap %s", vol.Name, pvname, bkpname)
	}

	if vol == nil {
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

		// set the volume status as pending
		rZV.Status.State = zfs.ZFSStatusPending

		// add original volume and schedule name in the label
		rZV.Labels = map[string]string{VeleroVolKey: pvname, VeleroNsKey: ns}
		rZV.Annotations = map[string]string{VeleroBkpKey: bkpname}

		vol, err = volbuilder.NewKubeclient().WithNamespace(p.namespace).Create(rZV)
		if err != nil {
			p.Log.Errorf("zfs: create ZFSVolume failed vol %v err: %v", rZV, err)
			return nil, err
		}

		err = p.checkVolCreation(rZV.Name)
		if err != nil {
			p.Log.Errorf("zfs: checkVolCreation failed %s err: %v", rZV.Name, err)
			return nil, err
		}
	} else {
		// this is incremental restore, update the ZFS volume
		vol.Spec = bkpZV.Spec
		vol, err := volbuilder.NewKubeclient().WithNamespace(p.namespace).Update(vol)
		if err != nil {
			p.Log.Errorf("zfs: update ZFSVolume failed vol %v err: %v", vol, err)
			return nil, err
		}
	}

	return vol, nil
}

func (p *Plugin) restoreZFSVolume(pvname, bkpname string) (*apis.ZFSVolume, error) {
	bkpZV := &apis.ZFSVolume{}

	filename := p.cl.GenerateRemoteFilename(pvname, bkpname)

	data, ok := p.cl.Read(filename + ".zfsvol")
	if !ok {
		return nil, errors.Errorf("zfs: failed to download ZFSVolume file=%s", filename+".zfsvol")
	}

	if err := json.Unmarshal(data, bkpZV); err != nil {
		return nil, errors.Errorf("zfs: failed to decode zfsvolume file=%s", filename+".zfsvol")
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

func (p *Plugin) checkRestoreStatus(rname, volname string) error {
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
			// delete the volume
			err = volbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(volname)
			if err != nil {
				// ignore error
				p.Log.Errorf("zfs: delete vol failed vol %s restore %s err: %v", volname, rname, err)
			}

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

// restoreVolume returns restored vol name and a boolean value indication if we need
// to restore the volume. If Volume is already restored, we don't need to restore it.
func (p *Plugin) restoreVolume(volname, bkpname string, port int) (string, string, error) {
	zv, err := p.restoreZFSVolume(volname, bkpname)
	if err != nil {
		p.Log.Errorf("zfs: restore ZFSVolume failed vol %s bkp %s err %v", volname, bkpname, err)
		return "", "", err
	}

	node := zv.Spec.OwnerNodeID
	serverAddr := p.remoteAddr + ":" + strconv.Itoa(port)
	zfsvol := zv.Name
	rname := utils.GenerateSnapshotID(zfsvol, bkpname)

	rstr, err := restorebuilder.NewBuilder().
		WithName(rname).
		WithVolume(zfsvol).
		WithNode(node).
		WithStatus(apis.RSTZFSStatusInit).
		WithRemote(serverAddr).
		Build()

	if err != nil {
		// delete the volume
		verr := volbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(zfsvol)
		if verr != nil {
			// ignore error
			p.Log.Errorf("zfs: delete vol failed vol %s rname %s err: %v", zfsvol, rname, verr)
		}
		return "", "", err
	}

	_, err = restorebuilder.NewKubeclient().WithNamespace(p.namespace).Create(rstr)

	if err != nil {
		// delete the volume
		verr := volbuilder.NewKubeclient().WithNamespace(p.namespace).Delete(zfsvol)
		if verr != nil {
			// ignore error
			p.Log.Errorf("zfs: delete vol failed vol %s rname %s err: %v", zfsvol, rname, err)
		}
		return "", "", err
	}
	return zfsvol, rname, nil
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

func (p *Plugin) doRestore(snapshotID string, port int) (string, error) {

	volname, bkpname, err := utils.GetInfoFromSnapshotID(snapshotID)
	if err != nil {
		return "", err
	}

	filename := p.cl.GenerateRemoteFilename(volname, bkpname)
	if filename == "" {
		return "", errors.Errorf("zfs: Error creating remote file name for restore")
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
		return "", errors.Errorf("zfs: restore server is not ready")
	}

	newvol, rname, err := p.restoreVolume(volname, bkpname, port)
	if err != nil {
		p.Log.Errorf("zfs: restoreVolume failed vol %s snap %s err: %v", volname, bkpname, err)
		return "", err
	}

	err = p.checkRestoreStatus(rname, newvol)
	if err != nil {
		p.Log.Errorf("zfs: restore failed vol %s snap %s err: %v", volname, bkpname, err)
		return "", err
	}

	p.Log.Debugf("zfs: restore done vol %s => %s snap %s", volname, newvol, snapshotID)
	return newvol, nil
}
