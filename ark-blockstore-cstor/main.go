package main

import (
	arkplugin "github.com/heptio/ark/pkg/plugin"
	snap "github.com/openebs/velero-plugin/pkg/snapshot"
	"github.com/sirupsen/logrus"
)

func main() {
	arkplugin.NewServer(arkplugin.NewLogger()).
		RegisterBlockStore("cstor-blockstore", openebsSnapPlugin).
		Serve()
}

func openebsSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &snap.BlockStore{Log: logger}, nil
}
