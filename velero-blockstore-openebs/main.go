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

package main

import (
	snap "github.com/openebs/velero-plugin/pkg/snapshot"
	zfssnap "github.com/openebs/velero-plugin/pkg/zfs/snapshot"
	"github.com/sirupsen/logrus"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	veleroplugin.NewServer().
		RegisterVolumeSnapshotter("openebs.io/cstor-blockstore", openebsSnapPlugin).
		RegisterVolumeSnapshotter("openebs.io/zfspv-blockstore", zfsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &snap.BlockStore{Log: logger}, nil
}

func zfsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &zfssnap.BlockStore{Log: logger}, nil
}
