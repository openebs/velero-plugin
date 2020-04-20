package main

import (
	snap "github.com/openebs/velero-plugin/pkg/snapshot"
	"github.com/sirupsen/logrus"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	veleroplugin.NewServer().
		RegisterVolumeSnapshotter("openebs.io/cstor-blockstore", openebsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &snap.BlockStore{Log: logger}, nil
}
