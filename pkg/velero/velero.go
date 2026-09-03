package velero

import (
	"os"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// kubeClient will be used to fetch velero custom resources
	kubeClient client.Client

	// veleroNs velero installation namespace
	veleroNs string
)

func init() {
	veleroNs = os.Getenv("VELERO_NAMESPACE")
}

// InitializeClientSet initialize velero clientset
func InitializeClientSet(config *rest.Config) error {
	scheme := runtime.NewScheme()
	if err := velerov1api.AddToScheme(scheme); err != nil {
		return err
	}

	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	kubeClient = c
	return nil
}
