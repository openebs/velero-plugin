package plugin

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/api/meta"
)

const (
	// STSAffinityGroupAnnotation is the key used by Mayastor to group PVCs for StatefulSets.
	STSAffinityGroupAnnotation = "openebs.io/stsAffinityGroup"
)

// RestorePlugin handles PVC restore operations for Mayastor volumes.
type RestorePlugin struct {
	log logrus.FieldLogger
}

// NewRestorePlugin returns an initialized RestorePlugin.
func NewRestorePlugin(log logrus.FieldLogger) *RestorePlugin {
	return &RestorePlugin{log: log}
}

// AppliesTo specifies which resources this plugin should apply to.
func (p *RestorePlugin) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"persistentvolumeclaims"},
	}, nil
}

// Execute modifies the 'openebs.io/stsAffinityGroup' annotation to reflect the restored namespace.
func (p *RestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	item := input.Item

	metadata, err := meta.Accessor(item)
	if err != nil {
		p.log.Errorf("Failed to access metadata for PVC: %v", err)
		return nil, fmt.Errorf("failed to access metadata for backed up PVC: %w", err)
	}

	originalNS := metadata.GetNamespace()
	restoreNS, ok := input.Restore.Spec.NamespaceMapping[originalNS]

	if !ok || restoreNS == "" {
		restoreNS = originalNS
	}

	if restoreNS == originalNS {
		p.log.Infof("Skipping PVC %s: namespace unchanged (%s)", metadata.GetName(), originalNS)
		return velero.NewRestoreItemActionExecuteOutput(item), nil
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	affinityVal, exists := annotations[STSAffinityGroupAnnotation]
	if !exists {
		p.log.Infof("Skipping PVC %q: annotation %q not found", metadata.GetName(), STSAffinityGroupAnnotation)
		return velero.NewRestoreItemActionExecuteOutput(item), nil
	}

	parts := strings.SplitN(affinityVal, "/", 2)
	if len(parts) != 2 {
		p.log.Errorf("invalid format for annotation %s:%s on PVC %s", STSAffinityGroupAnnotation, affinityVal, metadata.GetName())
		return nil, fmt.Errorf("invalid annotation format on PVC %s: %s", metadata.GetName(), affinityVal)
	}

	newAnnotation := fmt.Sprintf("%s/%s", restoreNS, parts[1])
	annotations[STSAffinityGroupAnnotation] = newAnnotation
	metadata.SetAnnotations(annotations)

	p.log.Infof("Updated annotation %s on PVC %s: %s -> %s", STSAffinityGroupAnnotation, metadata.GetName(), affinityVal, newAnnotation)

	return velero.NewRestoreItemActionExecuteOutput(item), nil
}
