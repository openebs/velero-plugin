package main

import (
	zfssnap "github.com/openebs/velero-plugin/pkg/zfs/snapshot"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	veleroplugin "github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

func main() {
	veleroplugin.NewServer().
		BindFlags(pflag.CommandLine).
		RegisterVolumeSnapshotter("openebs.io/zfspv-blockstore", zfsSnapPlugin).
		Serve()
}

func zfsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &zfssnap.BlockStore{Log: logger}, nil
}
