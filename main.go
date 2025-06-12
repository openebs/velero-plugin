package main

import (
	"github.com/openebs/velero-plugin/pkg/mayastor/plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

// main registers the custom restore plugin with Velero's framework.
func main() {
	framework.NewServer().
		BindFlags(pflag.CommandLine).
		RegisterRestoreItemAction("openebs.io/velero-plugin-mayastor", newRestorePlugin).
		Serve()
}

// newRestorePlugin initializes and returns an instance of the restore plugin.
func newRestorePlugin(logger logrus.FieldLogger) (interface{}, error) {
	return plugin.NewRestorePlugin(logger), nil
}
