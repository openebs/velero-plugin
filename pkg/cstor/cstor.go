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
	"net"
	"strconv"
	"strings"
	"time"

	cloud "github.com/openebs/velero-plugin/pkg/clouduploader"
	"github.com/pkg/errors"

	/* Due to dependency conflict, please ensure openebs
	 * dependency manually instead of using dep
	 */
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	openebs "github.com/openebs/maya/pkg/client/generated/clientset/versioned"
	velero "github.com/openebs/velero-plugin/pkg/velero"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	mayaAPIServiceName    = "maya-apiserver-service"
	mayaAPIServiceLabel   = "openebs.io/component-name=maya-apiserver-svc"
	backupEndpoint        = "/latest/backups/"
	restorePath           = "/latest/restore/"
	casTypeCStor          = "cstor"
	backupStatusInterval  = 5
	restoreStatusInterval = 5
	openebsVolumeLabel    = "openebs.io/cas-type"
)

const (
	// NAMESPACE config key for OpenEBS namespace
	NAMESPACE = "namespace"

	// LocalSnapshot config key for local snapshot
	LocalSnapshot = "local"

	// SnapshotIDIdentifier is a word to generate snapshotID from volume name and backup name
	SnapshotIDIdentifier = "-velero-bkp-"
)

// Plugin defines snapshot plugin for CStor volume
type Plugin struct {
	// Log is used for logging
	Log logrus.FieldLogger

	// K8sClient is used for kubernetes CR operation
	K8sClient *kubernetes.Clientset

	// OpenEBSClient is used for openEBS CR operation
	OpenEBSClient *openebs.Clientset

	// config to store parameters from velero server
	config map[string]string

	// namespace in which openebs is installed, default is openebs
	namespace string

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

	// if only local snapshot enabled
	local bool
}

// Snapshot describes snapshot object information
type Snapshot struct {
	// Volume name on which snapshot should be taken
	volID string

	// backupName is name of a snapshot
	backupName string

	// namespace is volume's namespace
	namespace string
}

// Volume describes volume object information
type Volume struct {
	// volname is volume name
	volname string

	// srcVolname is source volume name in case of local restore
	srcVolname string

	// namespace is volume claim's namespace
	namespace string

	// backupName is snapshot name for given volume
	backupName string

	// backupStatus is backup progress status for given volume
	backupStatus v1alpha1.CStorBackupStatus

	// restoreStatus is restore progress status for given volume
	restoreStatus v1alpha1.CStorRestoreStatus

	// size is volume size in string
	size resource.Quantity

	// snapshotTag is cloud snapshot file identifier.. It will be same as volume name from backup
	snapshotTag string

	// storageClass is volume's storageclass
	storageClass string

	iscsi v1.ISCSIPersistentVolumeSource
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

// Init CStor snapshot plugin
func (p *Plugin) Init(config map[string]string) error {
	if ns, ok := config[NAMESPACE]; ok {
		p.namespace = ns
	}

	conf, err := rest.InClusterConfig()
	if err != nil {
		p.Log.Errorf("Failed to get cluster config : %s", err.Error())
		return errors.New("error fetching cluster config")
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		p.Log.Errorf("Error creating clientset : %s", err.Error())
		return errors.New("error creating k8s client")
	}

	p.K8sClient = clientset

	openEBSClient, err := openebs.NewForConfig(conf)
	if err != nil {
		p.Log.Errorf("Failed to create openEBS client. %s", err)
		return err
	}
	p.OpenEBSClient = openEBSClient

	p.mayaAddr = p.getMapiAddr()
	if p.mayaAddr == "" {
		return errors.New("error fetching OpenEBS rest client address")
	}

	p.cstorServerAddr = p.getServerAddress()
	if p.cstorServerAddr == "" {
		return errors.New("error fetching cstorVeleroServer address")
	}
	p.config = config

	if p.volumes == nil {
		p.volumes = make(map[string]*Volume)
	}
	if p.snapshots == nil {
		p.snapshots = make(map[string]*Snapshot)
	}

	if local, ok := config[LocalSnapshot]; ok && local == "true" {
		p.local = true
		return nil
	}

	if err := velero.InitializeClientSet(conf); err != nil {
		return errors.Wrapf(err, "failed to initialize velero clientSet")
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

	// If PV doesn't have sufficient info to consider as CStor Volume
	// then we will return empty volumeId and error as nil.
	if pv.Name == "" ||
		pv.Spec.StorageClassName == "" ||
		(pv.Spec.ClaimRef != nil && pv.Spec.ClaimRef.Namespace == "") ||
		len(pv.Labels) == 0 {
		return "", nil
	}

	volType, ok := pv.Labels[openebsVolumeLabel]
	if !ok {
		return "", nil
	}

	if volType != casTypeCStor {
		return "", nil
	}

	if pv.Status.Phase == v1.VolumeReleased ||
		pv.Status.Phase == v1.VolumeFailed {
		return "", errors.New("pv is in released state")
	}

	if _, exists := p.volumes[pv.Name]; !exists {
		p.volumes[pv.Name] = &Volume{
			volname:      pv.Name,
			snapshotTag:  pv.Name,
			storageClass: pv.Spec.StorageClassName,
			namespace:    pv.Spec.ClaimRef.Namespace,
			size:         pv.Spec.Capacity[v1.ResourceStorage],
		}
	}

	return pv.Name, nil
}

// DeleteSnapshot delete CStor volume snapshot
func (p *Plugin) DeleteSnapshot(snapshotID string) error {
	var snapInfo *Snapshot
	var err error

	if snapshotID == "" {
		p.Log.Warning("Empty snapshotID")
		return nil
	}

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

	scheduleName := p.getScheduleName(snapInfo.backupName)

	if snapInfo.volID == "" || snapInfo.backupName == "" || snapInfo.namespace == "" || scheduleName == "" {
		return errors.Errorf("Got insufficient info vol:{%s} snap:{%s} ns:{%s} schedule:{%s}",
			snapInfo.volID,
			snapInfo.backupName,
			snapInfo.namespace,
			scheduleName)
	}

	err = p.sendDeleteRequest(snapInfo.backupName,
		snapInfo.volID,
		snapInfo.namespace,
		scheduleName)
	if err != nil {
		return errors.Wrapf(err, "failed to execute maya-apiserver DELETE API")
	}

	if p.local {
		// volumesnapshotlocation is configured for local snapshot
		return nil
	}

	filename := p.cl.GenerateRemoteFilename(snapInfo.volID, snapInfo.backupName)
	if filename == "" {
		return errors.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.Delete(filename)
	if !ret {
		return errors.New("failed to remove snapshot")
	}

	return nil
}

// CreateSnapshot creates snapshot for CStor volume and upload it to cloud storage
func (p *Plugin) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	if !p.local {
		p.cl.ExitServer = false
	}

	bkpname, ok := tags["velero.io/backup"]
	if !ok {
		return "", errors.New("failed to get backup name")
	}

	vol, ok := p.volumes[volumeID]
	if !ok {
		return "", errors.New("volume not found")
	}
	vol.backupName = bkpname
	size, ok := vol.size.AsInt64()
	if !ok {
		return "", errors.Errorf("Failed to parse volume size %v", vol.size)
	}

	if !p.local {
		// If cloud snapshot is configured then we need to backup PVC also
		err := p.backupPVC(volumeID)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create backup for PVC")
		}
	}

	p.Log.Infof("creating snapshot{%s}", bkpname)

	bkp, err := p.sendBackupRequest(vol)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to send backup request")
	}

	p.Log.Infof("Snapshot Successfully Created")

	if p.local {
		// local snapshot
		return generateSnapshotID(volumeID, bkpname), nil
	}

	filename := p.cl.GenerateRemoteFilename(vol.snapshotTag, vol.backupName)
	if filename == "" {
		return "", errors.Errorf("Error creating remote file name for backup")
	}

	go p.checkBackupStatus(bkp)

	ok = p.cl.Upload(filename, size)
	if !ok {
		return "", errors.New("failed to upload snapshot")
	}

	if vol.backupStatus == v1alpha1.BKPCStorStatusDone {
		return generateSnapshotID(volumeID, bkpname), nil
	}

	return "", errors.Errorf("Failed to upload snapshot, status:{%v}", vol.backupStatus)
}

func (p *Plugin) getSnapInfo(snapshotID string) (*Snapshot, error) {
	volumeID, bkpName, err := getInfoFromSnapshotID(snapshotID)
	if err != nil {
		return nil, err
	}

	pv, err := p.K8sClient.
		CoreV1().
		PersistentVolumes().
		Get(volumeID, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Errorf("Error fetching volume{%s} : %s", volumeID, err.Error())
	}

	// TODO
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
	var (
		newVol *Volume
		err    error
	)

	if volumeType != "cstor-snapshot" {
		return "", errors.Errorf("Invalid volume type{%s}", volumeType)
	}

	volumeID, snapName, err := getInfoFromSnapshotID(snapshotID)
	if err != nil {
		return "", err
	}

	snapType := "remote"
	if p.local {
		snapType = "local"
	}

	p.Log.Infof("Restoring %s snapshot{%s} for volume:%s", snapType, snapName, volumeID)

	if p.local {
		newVol, err = p.getVolumeForLocalRestore(volumeID, snapName)
		if err != nil {
			return "", errors.Wrapf(err, "Failed to read PVC for volumeID=%s snap=%s", volumeID, snapName)
		}

		err = p.restoreVolumeFromLocal(newVol)
	} else {
		newVol, err = p.getVolumeForRemoteRestore(volumeID, snapName)
		if err != nil {
			return "", errors.Wrapf(err, "Failed to read PVC for volumeID=%s snap=%s", volumeID, snapName)
		}

		err = p.restoreVolumeFromCloud(newVol)
	}

	if err != nil {
		p.Log.Errorf("Failed to restore volume : %s", err)
		return "", errors.Wrapf(err, "Failed to restore volume")
	}

	if newVol.restoreStatus == v1alpha1.RSTCStorStatusDone {
		p.Log.Infof("Restore completed for CStor volume:%s snapshot:%s", volumeID, snapName)
		return newVol.volname, nil
	}

	return "", errors.New("failed to restore snapshot")
}

// GetVolumeInfo return volume information for given volume name
func (p *Plugin) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return "cstor-snapshot", nil, nil
}

// SetVolumeID set volumeID for given PV
func (p *Plugin) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	pv := new(v1.PersistentVolume)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	vol := p.volumes[volumeID]

	if p.local {
		fsType := pv.Spec.PersistentVolumeSource.ISCSI.FSType

		pv.Spec.PersistentVolumeSource = v1.PersistentVolumeSource{
			ISCSI: &vol.iscsi,
		}

		// Set Old PV fsType
		pv.Spec.PersistentVolumeSource.ISCSI.FSType = fsType
	}

	pv.Name = vol.volname

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}

// getScheduleName return the schedule name for the given backup
// It will check if backup name have 'bkp-20060102150405' format
func (p *Plugin) getScheduleName(backupName string) string {
	// for non-scheduled backup, we are considering backup name as schedule name only
	scheduleOrBackupName := backupName

	// If it is scheduled backup then we need to get the schedule name
	splitName := strings.Split(backupName, "-")
	if len(splitName) >= 2 {
		_, err := time.Parse("20060102150405", splitName[len(splitName)-1])
		if err != nil {
			// last substring is not timestamp, so it is not generated from schedule
			return scheduleOrBackupName
		}
		scheduleOrBackupName = strings.Join(splitName[0:len(splitName)-1], "-")
	}
	return scheduleOrBackupName
}

// getInfoFromSnapshotID return backup name and volume id from the given snapshotID
func getInfoFromSnapshotID(snapshotID string) (volumeID, backupName string, err error) {
	s := strings.Split(snapshotID, SnapshotIDIdentifier)
	if len(s) != 2 {
		err = errors.New("invalid snapshot id")
		return
	}

	volumeID = s[0]
	backupName = s[1]

	if volumeID == "" || backupName == "" {
		err = errors.Errorf("invalid volumeID=%s backupName=%s", volumeID, backupName)
	}
	return
}

func generateSnapshotID(volumeID, backupName string) string {
	return volumeID + SnapshotIDIdentifier + backupName
}
