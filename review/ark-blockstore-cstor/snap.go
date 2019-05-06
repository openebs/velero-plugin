package main

import (
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// Plugin for containing state for the blockstore plugin
type BlockStore struct {
	Log logrus.FieldLogger
	plugin cloudprovider.BlockStore
}

var _ cloudprovider.BlockStore = (*BlockStore)(nil)

// Init the plugin
func (p *BlockStore) Init(config map[string]string) error {
	p.Log.Infof("Initializing ark plugin for cstor %v", config)

	/* As of now, only cStore volumes are supported */
	p.plugin = &cstorSnapPlugin{Log: p.Log}
	return p.plugin.Init(config)
}

// CreateVolumeFromSnapshot Create a volume form given snapshot
func (p *BlockStore) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	return p.plugin.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
}

// GetVolumeInfo Get information about the volume
func (p *BlockStore) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return p.plugin.GetVolumeInfo(volumeID, volumeAZ)
}

// IsVolumeReady Check if the volume is ready.
func (p *BlockStore) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	return true, nil
}

// CreateSnapshot Create a snapshot
func (p *BlockStore) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	return p.plugin.CreateSnapshot(volumeID, volumeAZ, tags)
}

// DeleteSnapshot Delete a snapshot
func (p *BlockStore) DeleteSnapshot(snapshotID string) error {
	return p.plugin.DeleteSnapshot(snapshotID)
}

// GetVolumeID Get the volume ID from the spec
func (p *BlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	return p.plugin.GetVolumeID(pv)
}

// SetVolumeID Set the volume ID in the spec
func (p *BlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	return p.plugin.SetVolumeID(pv, volumeID)
}
