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

package snapshot

import (
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/openebs/velero-plugin/pkg/cstor"
	"github.com/sirupsen/logrus"
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
	p.Log.Infof("Initializing velero plugin for CStor %v", config)

	// TODO check for type of volume
	p.plugin = &cstor.Plugin{Log: p.Log}
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
