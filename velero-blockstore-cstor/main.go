package main

import (
	veleroplugin "github.com/heptio/velero/pkg/plugin"
	snap "github.com/openebs/velero-plugin/pkg/snapshot"
	"github.com/sirupsen/logrus"
)

func main() {
	veleroplugin.NewServer(veleroplugin.NewLogger()).
		RegisterBlockStore("cstor-blockstore", openebsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &snap.BlockStore{Log: logger}, nil
}
