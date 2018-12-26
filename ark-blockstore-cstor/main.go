package main

import (
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/sirupsen/logrus"
)

func main() {
	arkplugin.NewServer(arkplugin.NewLogger()).
		RegisterBlockStore("cstor-blockstore", cstorSnapPlugin).
		Serve()
}

func cstorSnapPlugin(logger logrus.FieldLogger) (interface{}, error) {
	return &BlockStore{Log: logger}, nil
}
