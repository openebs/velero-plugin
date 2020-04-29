package velero

import (
	"os"

	veleroclient "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	"k8s.io/client-go/rest"
)

var (
	// clientSet will be used to fetch velero customo resources
	clientSet veleroclient.Interface

	// veleroNs velero installation namespace
	veleroNs string
)

func init() {
	veleroNs = os.Getenv("VELERO_NAMESPACE")
}

// InitializeClientSet initialize velero clientset
func InitializeClientSet(config *rest.Config) error {
	var err error

	clientSet, err = veleroclient.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}
