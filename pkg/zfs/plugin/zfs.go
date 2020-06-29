/*
Copyright 2020 the Velero contributors.

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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/openebs/velero-plugin/pkg/zfs/utils"
	"github.com/openebs/velero-plugin/pkg/velero"
	cloud "github.com/openebs/velero-plugin/pkg/clouduploader"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// ZFSPV_NAMESPACE config key for OpenEBS namespace
	ZFSPV_NAMESPACE = "namespace"
	// ZFSPV_BACKUP config key for backup type full or incremental
	ZFSPV_BACKUP = "backup"
	backupStatusInterval  = 5
)

// Plugin is a plugin for containing state for the blockstore
type Plugin struct {
	config map[string]string
	Log logrus.FieldLogger

	// K8sClient is used for kubernetes operation
	K8sClient *kubernetes.Clientset

	// on this address cloud server will perform data operation(backup/restore)
	remoteAddr string

	// this is the namespace where all the ZFSPV CRs will be created,
	// this should be same as what is passed to ZFS-LocalPV driver
	// as env OPENEBS_NAMESPACE while deploying it.
	namespace string

	// This specify whether we have to take incremental backup or full backup
	incremental bool

	// cl stores cloud connection information
	cl *cloud.Conn
}

// Init prepares the VolumeSnapshotter for usage using the provided map of
// configuration key-value pairs. It returns an error if the VolumeSnapshotter
// cannot be initialized from the provided config. Note that after v0.10.0, this will happen multiple times.
func (p *Plugin) Init(config map[string]string) error {
	p.Log.Infof("zfs: Init called %v", config)
	p.config = config

	p.remoteAddr, _  = utils.GetServerAddress()
	if p.remoteAddr == "" {
		return errors.New("zfs: error fetching Server address")
	}

	if ns, ok := config[ZFSPV_NAMESPACE]; ok {
		p.namespace = ns
	} else {
		p.namespace = "openebs" // default namespace
	}

	if bkptype, ok := config[ZFSPV_BACKUP]; ok && bkptype == "incremental" {
		p.incremental = true
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

	if err := velero.InitializeClientSet(conf); err != nil {
		return errors.Wrapf(err, "failed to initialize velero clientSet")
	}

	p.K8sClient = clientset

	p.cl = &cloud.Conn{Log: p.Log}
	return p.cl.Init(config)
}

// CreateVolumeFromSnapshot creates a new volume from the specified snapshot
func (p *Plugin) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	p.Log.Infof("zfs: CreateVolumeFromSnapshot called snap %s", snapshotID)

	volumeID, err := p.doRestore(snapshotID)

	if err != nil {
		p.Log.Errorf("zfs: error CreateVolumeFromSnapshot returning snap %s err %v", snapshotID, err)
		return "", err
	}

	p.Log.Infof("zfs: CreateVolumeFromSnapshot returning snap %s vol %s", snapshotID, volumeID)
	return volumeID, nil
}

// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for
// the specified volume in the given availability zone.
func (p *Plugin) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	p.Log.Infof("zfs: GetVolumeInfo called", volumeID, volumeAZ)
	return "zfs-localpv", nil, nil
}

// IsVolumeReady Check if the volume is ready.
func (p *Plugin) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	p.Log.Infof("zfs: IsVolumeReady called", volumeID, volumeAZ)

	return p.isVolumeReady(volumeID)
}

// CreateSnapshot creates a snapshot of the specified volume, and applies any provided
// set of tags to the snapshot.
func (p *Plugin) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	p.Log.Infof("zfs: CreateSnapshot called", volumeID, volumeAZ, tags)

	bkpname, ok := tags[VeleroBkpKey]
	if !ok {
		return "", errors.New("zfs: error get backup name")
	}

	schdname, ok := tags[VeleroSchdKey]

	snapshotID, err := p.doBackup(volumeID, bkpname, schdname)

	if err != nil {
		p.Log.Errorf("zfs: error createBackup %s@%s failed %v", volumeID, bkpname, err)
		return "", err
	}

	p.Log.Infof("zfs: CreateSnapshot returning %s", snapshotID)
	return snapshotID, nil
}

// DeleteSnapshot deletes the specified volume snapshot.
func (p *Plugin) DeleteSnapshot(snapshotID string) error {
	p.Log.Infof("zfs: DeleteSnapshot called %s", snapshotID)
	if snapshotID == "" {
		p.Log.Warning("zfs: Empty snapshotID")
		return nil
	}

	return p.deleteBackup(snapshotID)
}

// GetVolumeID returns the specific identifier for the PersistentVolume.
func (p *Plugin) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	p.Log.Infof("zfs: GetVolumeID called %v", unstructuredPV)

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return "", errors.WithStack(err)
	}

	return pv.Name, nil
}

// SetVolumeID sets the specific identifier for the PersistentVolume.
func (p *Plugin) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	p.Log.Infof("zfs: SetVolumeID called %v %s", unstructuredPV, volumeID)

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	// Set the PV Name and VolumeHandle
	pv.Name = volumeID
	pv.Spec.PersistentVolumeSource.CSI.VolumeHandle = volumeID

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}
