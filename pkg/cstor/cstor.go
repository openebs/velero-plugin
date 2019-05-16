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
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	cloud "github.com/openebs/velero-plugin/pkg/clouduploader"
	"github.com/pkg/errors"
	/* Due to dependency conflict, please ensure openebs
	 * dependency manually instead of using dep
	 */
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	mayaAPIServiceName    = "maya-apiserver-service"
	backupEndpoint        = "/latest/backups/"
	restorePath           = "/latest/restore/"
	operator              = "openebs"
	casType               = "cstor"
	backupStatusInterval  = 5
	restoreStatusInterval = 5
)

// Plugin defines snapshot plugin for CStor volume
type Plugin struct {
	// Log is used for logging
	Log logrus.FieldLogger

	// K8sClient is used for kubernetes CR operation
	K8sClient corev1.CoreV1Interface

	// config to store parameters from velero server
	config map[string]string

	// cl stores cloud connection information
	cl *cloud.Conn

	// mayaAddr is maya API server address
	mayaAddr string

	// cstorServerAddr is network address used for CStor volume operation
	// on this address cloud server will perform data operation(backup/restore)
	cstorServerAddr string

	// volumes list of volume
	volumes map[string]*Volume

	// snapshots list of snapshot
	snapshots map[string]*Snapshot
}

// Snapshot describes snapshot object information
type Snapshot struct {
	//Volume name on which snapshot should be taken
	volID string

	//backupName is name of a snapshot
	backupName string

	//namespace is volume's namespace
	namespace string
}

// Volume describes volume object information
type Volume struct {
	//volname is volume name
	volname string

	//casType is volume type
	casType string

	//namespace is volume's namespace
	namespace string

	//backupName is snapshot name for given volume
	backupName string

	//backupStatus is backup progress status for given volume
	backupStatus v1alpha1.CStorBackupStatus

	//restoreStatus is restore progress status for given volume
	restoreStatus v1alpha1.CStorRestoreStatus
}

func (p *Plugin) getServerAddress() string {
	netInterfaceAddresses, err := net.InterfaceAddrs()

	if err != nil {
		p.Log.Errorf("Failed to get interface Address for velero server : %s", err.Error())
		return ""
	}

	for _, netInterfaceAddress := range netInterfaceAddresses {
		networkIP, ok := netInterfaceAddress.(*net.IPNet)
		if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
			ip := networkIP.IP.String()
			p.Log.Infof("Ip address of velero-plugin server: %s", ip)
			return ip + ":" + strconv.Itoa(cloud.RecieverPort)
		}
	}
	return ""
}

// getMapiAddr return maya API server's ip address
func (p *Plugin) getMapiAddr() string {
	sc, err := p.K8sClient.Services(operator).Get(mayaAPIServiceName, metav1.GetOptions{})
	if err != nil {
		p.Log.Errorf("Error getting IP Address for service{%s} : %v", mayaAPIServiceName, err.Error())
		return ""
	}

	if len(sc.Spec.ClusterIP) != 0 {
		return "http://" + sc.Spec.ClusterIP + ":" + strconv.FormatInt(int64(sc.Spec.Ports[0].Port), 10)
	}
	return ""
}

// Init CStor snapshot plugin
func (p *Plugin) Init(config map[string]string) error {
	conf, err := rest.InClusterConfig()
	if err != nil {
		p.Log.Errorf("Failed to get cluster config : %s", err.Error())
		return errors.New("Error fetching cluster config")
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		p.Log.Errorf("Error creating clientset : %s", err.Error())
		return errors.New("Error creating k8s client")
	}

	p.K8sClient = clientset.CoreV1()
	p.mayaAddr = p.getMapiAddr()
	if p.mayaAddr == "" {
		return errors.New("Error fetching OpenEBS rest client address")
	}

	p.cstorServerAddr = p.getServerAddress()
	if p.cstorServerAddr == "" {
		return errors.New("Error fetch cstorVeleroServer address")
	}
	p.config = config
	if p.volumes == nil {
		p.volumes = make(map[string]*Volume)
	}
	if p.snapshots == nil {
		p.snapshots = make(map[string]*Snapshot)
	}

	p.cl = &cloud.Conn{Log: p.Log}
	return p.cl.Init(config)
}

// GetVolumeID return volume name for given PV
func (p *Plugin) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	pv := new(v1.PersistentVolume)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return "", errors.WithStack(err)
	}

	if pv.Name == "" || pv.Spec.StorageClassName == "" || (pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace == "") {
		p.Log.Errorf("Insufficient info for PV : %v", pv)
		return "", errors.New("Insufficient info for PV")
	}

	if _, exists := p.volumes[pv.Name]; !exists {
		p.volumes[pv.Name] = &Volume{
			volname:   pv.Name,
			casType:   pv.Spec.StorageClassName,
			namespace: pv.Spec.ClaimRef.Namespace,
		}
	}

	return pv.Name, nil
}

// DeleteSnapshot delete CStor volume snapshot
func (p *Plugin) DeleteSnapshot(snapshotID string) error {
	var snapInfo *Snapshot
	var err error

	p.Log.Infof("Deleting snapshot %v", snapshotID)
	if _, exists := p.snapshots[snapshotID]; !exists {
		snapInfo, err = p.getSnapInfo(snapshotID)
		if err != nil {
			return err
		}
		p.snapshots[snapshotID] = snapInfo
	} else {
		snapInfo = p.snapshots[snapshotID]
	}

	if snapInfo.volID == "" || snapInfo.backupName == "" || snapInfo.namespace == "" {
		return errors.Errorf("Got insufficient info vol:{%s} snap:{%s} ns:{%s}",
			snapInfo.volID,
			snapInfo.backupName,
			snapInfo.namespace)
	}

	url := p.mayaAddr + backupEndpoint + snapInfo.backupName

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		p.Log.Errorf("Failed to create HTTP request")
		return err
	}

	q := req.URL.Query()
	q.Add("volume", snapInfo.volID)
	q.Add("namespace", snapInfo.namespace)
	q.Add("casType", casType)

	req.URL.RawQuery = q.Encode()

	c := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := c.Do(req)
	if err != nil {
		return errors.Errorf("Error when connecting to maya-apiserver : %s", err.Error())
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.Log.Warnf("Failed to close response : %s", err.Error())
		}
	}()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Errorf("Unable to read response from maya-apiserver : %s", err.Error())
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return errors.Errorf("HTTP Status error{%v} from maya-apiserver", code)
	}

	filename := p.cl.GenerateRemoteFilename(snapInfo.volID, snapInfo.backupName)
	if filename == "" {
		return errors.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.Delete(filename)
	if !ret {
		return errors.New("Failed to remove snapshot")
	}

	return nil
}

// CreateSnapshot creates snapshot for CStor volume and upload it to cloud storage
func (p *Plugin) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	var vol *Volume

	p.cl.ExitServer = false
	bkpname, ret := tags["velero.io/backup"]
	if !ret {
		return "", errors.New("Failed to get backup name")
	}

	if _, ret := p.volumes[volumeID]; !ret {
		return "", errors.New("Volume is not found")
	}

	vol = p.volumes[volumeID]
	vol.backupName = bkpname
	err := p.backupPVC(volumeID)
	if err != nil {
		return "", errors.New("failed to create backup for PVC")
	}

	p.Log.Infof("creating snapshot{%s}", bkpname)

	splitName := strings.Split(bkpname, "-")
	bname := ""
	if len(splitName) >= 2 {
		bname = strings.Join(splitName[0:len(splitName)-1], "-")
	} else {
		bname = bkpname
	}

	if len(strings.TrimSpace(bkpname)) == 0 {
		return "", errors.New("No bkpname")
	}

	bkpSpec := &v1alpha1.CStorBackupSpec{
		BackupName: bname,
		VolumeName: volumeID,
		SnapName:   bkpname,
		BackupDest: p.cstorServerAddr,
	}

	bkp := &v1alpha1.CStorBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: vol.namespace,
		},
		Spec: *bkpSpec,
	}

	url := p.mayaAddr + backupEndpoint

	bkpData, err := json.Marshal(bkp)
	if err != nil {
		p.Log.Errorf("Error during JSON marshal : %s", err.Error())
		return "", err
	}

	_, err = p.httpRestCall(url, "POST", bkpData)
	if err != nil {
		return "", errors.Errorf("Error calling REST api : %s", err.Error())
	}

	p.Log.Infof("Snapshot Successfully Created")
	filename := p.cl.GenerateRemoteFilename(volumeID, vol.backupName)
	if filename == "" {
		return "", errors.Errorf("Error creating remote file name for backup")
	}

	go p.checkBackupStatus(bkp)

	ret = p.cl.Upload(filename)
	if !ret {
		return "", errors.New("Failed to upload snapshot")
	}

	if vol.backupStatus == v1alpha1.BKPCStorStatusDone {
		return volumeID + "-velero-bkp-" + bkpname, nil
	}
	return "", errors.Errorf("Failed to upload snapshot, status:{%v}", vol.backupStatus)
}

func (p *Plugin) getSnapInfo(snapshotID string) (*Snapshot, error) {
	s := strings.Split(snapshotID, "-velero-bkp-")
	volumeID := s[0]
	bkpName := s[1]

	pv, err := p.K8sClient.PersistentVolumes().Get(snapshotID, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Errorf("Error fetching namespaces for volume{%s} : %s", volumeID, err.Error())
	}

	if pv.Spec.ClaimRef.Namespace == "" {
		return nil, errors.Errorf("No namespace in pv.spec.claimref for PV{%s}", snapshotID)

	}
	return &Snapshot{
		volID:      volumeID,
		backupName: bkpName,
		namespace:  pv.Spec.ClaimRef.Namespace,
	}, nil
}

// CreateVolumeFromSnapshot create CStor volume for given
// snapshotID and perform restore operation on it
func (p *Plugin) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	p.cl.ExitServer = false
	if volumeType != "cstor-snapshot" {
		return "", errors.Errorf("Invalid volume type{%s}", volumeType)
	}

	s := strings.Split(snapshotID, "-velero-bkp-")
	volumeID := s[0]
	snapName := s[1]

	p.Log.Infof("Restoring snapshot{%s} for volume:%s", snapName, volumeID)

	newVol, e := p.createPVC(volumeID, snapName)
	if e != nil {
		return "", errors.Errorf("Failed to restore PVC")
	}

	p.Log.Infof("New volume(%v) created", newVol)

	restoreSpec := &v1alpha1.CStorRestoreSpec{
		RestoreName: newVol.backupName,
		VolumeName:  newVol.volname,
		RestoreSrc:  p.cstorServerAddr,
	}

	restore := &v1alpha1.CStorRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: newVol.namespace,
		},
		Spec: *restoreSpec,
	}

	url := p.mayaAddr + restorePath

	restoreData, err := json.Marshal(restore)
	if err != nil {
		p.Log.Errorf("Error during JSON marshal : %s", err.Error())
		return "", err
	}

	if _, err := p.httpRestCall(url, "POST", restoreData); err != nil {
		p.Log.Errorf("Error executing REST api : %s", err.Error())
		return "", errors.Errorf("Error executing REST api for restore : %s", err.Error())
	}

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)
	if filename == "" {
		p.Log.Errorf("Error failed to create remote file-name for backup")
		return "", errors.Errorf("Error creating remote file name for backup")
	}

	go p.checkRestoreStatus(restore, newVol)

	ret := p.cl.Download(filename)
	if !ret {
		p.Log.Errorf("Failed to restore snapshot")
		return "", errors.New("Failed to restore snapshot")
	}

	if newVol.restoreStatus == v1alpha1.RSTCStorStatusDone {
		p.Log.Infof("Restore completed")
		return volumeID, nil
	}

	return "", errors.New("Failed to restore snapshot")
}

// GetVolumeInfo return volume information for given volume name
func (p *Plugin) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return "cstor-snapshot", nil, nil
}

// createPVC  create PVC for given volume name
func (p *Plugin) createPVC(volumeID, snapName string) (*Volume, error) {
	var pvc v1.PersistentVolumeClaim
	var data []byte
	var ok bool

	filename := p.cl.GenerateRemoteFilename(volumeID, snapName)
	if filename == "" {
		return nil, errors.New("Error creating remote file name for pvc backup")
	}

	if data, ok = p.cl.Read(filename + ".pvc"); !ok {
		return nil, errors.New("Failed to download PVC")
	}

	if err := json.Unmarshal(data, &pvc); err != nil {
		return nil, errors.New("Failed to decode pvc")
	}

	newVol, err := p.getVolumeFromPVC(pvc)
	if err == nil {
		newVol.backupName = snapName
		return newVol, nil
	}

	pvc.Annotations = make(map[string]string)
	pvc.Annotations["openebs.io/created-through"] = "restore"
	rpvc, er := p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Create(&pvc)
	if er != nil {
		return nil, errors.Errorf("Failed to create PVC : %s", er.Error())
	}

	for {
		pvc, er := p.K8sClient.PersistentVolumeClaims(rpvc.Namespace).Get(rpvc.Name, metav1.GetOptions{})
		if er != nil || pvc.Status.Phase == v1.ClaimLost {
			if err := p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Delete(rpvc.Name, nil); err != nil {
				p.Log.Warnf("Failed to delete pvc {%s} : %s", rpvc.Name, err.Error())
			}
			return nil, errors.Errorf("Failed to create PVC : %s", er.Error())
		}
		if pvc.Status.Phase == v1.ClaimBound {
			p.Log.Infof("PVC(%v) created..", pvc.Name)
			return &Volume{
				volname:    pvc.Spec.VolumeName,
				namespace:  pvc.Namespace,
				backupName: snapName,
				casType:    *pvc.Spec.StorageClassName,
			}, nil
		}
	}
}

// backupPVC perform backup for given volume's PVC
func (p *Plugin) backupPVC(volumeID string) error {
	vol := p.volumes[volumeID]
	var bkpPvc *v1.PersistentVolumeClaim

	pvcs, err := p.K8sClient.PersistentVolumeClaims(vol.namespace).List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorf("Error fetching PVC list : %s", err.Error())
		return errors.New("Failed to fetch PVC list")
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == vol.volname {
			bkpPvc = &pvc
			break
		}
	}

	if bkpPvc == nil {
		p.Log.Errorf("Failed to find PVC for PV{%s}", vol.volname)
		return errors.Errorf("Failed to find PVC for volume{%s}", vol.volname)
	}

	bkpPvc.ResourceVersion = ""
	bkpPvc.SelfLink = ""
	bkpPvc.Annotations = nil
	bkpPvc.UID = ""
	bkpPvc.Spec.VolumeName = ""

	data, err := json.MarshalIndent(bkpPvc, "", "\t")
	if err != nil {
		return errors.New("Error doing json parsing")
	}

	filename := p.cl.GenerateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("Error creating remote file name for pvc backup")
	}

	if ok := p.cl.Write(data, filename+".pvc"); !ok {
		return errors.New("Failed to upload PVC")
	}

	return nil
}

// SetVolumeID set volumeID for given PV
func (p *Plugin) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	pv := new(v1.PersistentVolume)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	// We will not update HostPath since CStor volume doesn't have one
	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}

// httpRestCall execute REST API
func (p *Plugin) httpRestCall(url, reqtype string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(reqtype, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, errors.Errorf("Error when connecting to maya-apiserver : %s", err.Error())
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.Log.Warnf("Failed to close response : %s", err.Error())
		}
	}()

	respdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("Unable to read response from maya-apiserver : %s", err.Error())
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return nil, errors.Errorf("Status error{%v}", http.StatusText(code))
	}
	return respdata, nil
}

// getVolumeFromPVC returns volume info for given PVC if PVC is in bound state
func (p *Plugin) getVolumeFromPVC(pvc v1.PersistentVolumeClaim) (*Volume, error) {
	rpvc, err := p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Errorf("PVC{%s} does not exist", pvc.Name)
	}

	if rpvc.Status.Phase == v1.ClaimLost {
		p.Log.Errorf("PVC{%s} is not bound yet!", rpvc.Name)
		panic(errors.Errorf("PVC{%s} is not bound yet", rpvc.Name))
	} else {
		return &Volume{
			volname:   rpvc.Spec.VolumeName,
			namespace: rpvc.Namespace,
			casType:   *rpvc.Spec.StorageClassName,
		}, nil
	}
}

// checkBackupStatus queries MayaAPI server for given backup status
// and wait until backup completes
func (p *Plugin) checkBackupStatus(bkp *v1alpha1.CStorBackup) {
	var bkpdone bool
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

	for !bkpdone {
		time.Sleep(backupStatusInterval * time.Second)
		var bs v1alpha1.CStorBackup

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
			bkpdone = true
			p.cl.ExitServer = true
		}
	}
}

// checkRestoreStatus queries MayaAPI server for given restore status
// and wait until restore completes
func (p *Plugin) checkRestoreStatus(rst *v1alpha1.CStorRestore, vol *Volume) {
	var rstdone bool
	url := p.mayaAddr + restorePath

	rstData, err := json.Marshal(rst)
	if err != nil {
		p.Log.Errorf("JSON marshal failed : %s", err.Error())
		panic(errors.Errorf("JSON marshal failed : %s", err.Error()))
	}

	for !rstdone {
		time.Sleep(restoreStatusInterval * time.Second)
		var rs v1alpha1.CStorRestore

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
			rstdone = true
			p.cl.ExitServer = true
		}
	}
}
