package main

import (
	"errors"
	"fmt"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// Volume keeps track of volumes created by this plugin
type Volume struct {
	volType, az, namespace, pvc string
}

// Plugin for containing state for the blockstore plugin
type BlockStore struct {
	config map[string]string
	Log logrus.FieldLogger
	volumes   map[string]Volume
	snapshots map[string]Snapshot
}

var _ cloudprovider.BlockStore = (*BlockStore)(nil)
var cstorIf cstorSnap

// Init the plugin
func (p *BlockStore) Init(config map[string]string) error {
	p.Log.Infof("Initializing ark plugin for cstor %v", config)
	p.config = config

	cstorIf.Log = p.Log

	// Make sure we don't overwrite data, now that we can re-initialize the plugin
	if p.volumes == nil {
		p.volumes = make(map[string]Volume)
	}
	if p.snapshots == nil {
		p.snapshots = make(map[string]Snapshot)
	}

	return nil
}

// CreateVolumeFromSnapshot Create a volume form given snapshot
func (p *BlockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	if volumeType != "cstor-snapshot" {
		return "", fmt.Errorf("Invalid volume type(%s)", volumeType)
	}

	volumeID, resp := cstorIf.createVolume(snapshotID, p.config)
	if resp != nil {
		return "", fmt.Errorf("Failed to create restore:%s", resp)
	}

	return volumeID, fmt.Errorf("Failed to restore")
//	return volumeID, nil
}

// GetVolumeInfo Get information about the volume
func (p *BlockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return "cstor-snapshot", nil, nil
}

// IsVolumeReady Check if the volume is ready.
func (p *BlockStore) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	return true, nil
}

// CreateSnapshot Create a snapshot
func (p *BlockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	ns := p.volumes[volumeID].namespace

	bkpname, terr := tags["ark.heptio.com/backup"]
        if terr != true {
                return "",  errors.New("Failed to get backup name")
        }

	p.Log.Infof("creating snapshot", bkpname)
	resp := cstorIf.snapCreateReq(volumeID, bkpname, ns, p.config)
	if resp != nil {
		return "", fmt.Errorf("Failed to create backup:%s", resp)
	}

	return volumeID + "-ark-bkp-" + bkpname, nil
}

// DeleteSnapshot Delete a snapshot
func (p *BlockStore) DeleteSnapshot(snapshotID string) error {
	var snapInfo *Snapshot
	var err error

	p.Log.Infof("Deleting snapshot", snapshotID)
	if _, exists := p.snapshots[snapshotID]; !exists {
		snapInfo, err = cstorIf.getSnapInfo(snapshotID)
		if err != nil {
			return err
		}
		p.snapshots[snapshotID] = *snapInfo
	} else {
		*snapInfo = p.snapshots[snapshotID]
	}

	return cstorIf.snapDeleteReq(p.snapshots[snapshotID], p.config)
}

// GetVolumeID Get the volume ID from the spec
func (p *BlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "metadata") {
		return "", nil
	}

	// Seed the volume info so that GetVolumeInfo doesn't fail later.
	volumeID, _ := collections.GetString(pv.UnstructuredContent(), "metadata.name")
	if _, exists := p.volumes[volumeID]; !exists {
		sc, _ := collections.GetString(pv.UnstructuredContent(), "spec.storageClassName")
		ns, _ := collections.GetString(pv.UnstructuredContent(), "spec.claimRef.namespace")
		p.volumes[volumeID] = Volume{
			volType: sc,
			namespace: ns,
			pvc: volumeID,
		}
	}

	return collections.GetString(pv.UnstructuredContent(), "metadata.name")
}

// SetVolumeID Set the volume ID in the spec
func (p *BlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	metadataMap, err := collections.GetMap(pv.UnstructuredContent(), "spec.hostPath.path")
	if err != nil {
		return nil, err
	}

	metadataMap["volumeID"] = volumeID
	return pv, nil
}
