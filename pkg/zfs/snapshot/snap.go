package snapshot

import (
	zfs "github.com/openebs/velero-plugin/pkg/zfs/plugin"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/runtime"
)

// BlockStore : Plugin for containing state for the blockstore plugin
type BlockStore struct {
	Log    logrus.FieldLogger
	plugin velero.VolumeSnapshotter
}

var _ velero.VolumeSnapshotter = (*BlockStore)(nil)

// Init the plugin
func (p *BlockStore) Init(config map[string]string) error {
	p.Log.Infof("zfs: Initializing velero plugin for ZFS-LocalPV")

	p.plugin = &zfs.Plugin{Log: p.Log}
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
func (p *BlockStore) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	return p.plugin.GetVolumeID(unstructuredPV)
}

// SetVolumeID Set the volume ID in the spec
func (p *BlockStore) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	return p.plugin.SetVolumeID(unstructuredPV, volumeID)
}
