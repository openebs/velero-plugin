package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const srcAffinityGroup = "src/sts"

func newPVC(ns string, annotations map[string]interface{}) *unstructured.Unstructured {
	meta := map[string]interface{}{
		"name":      "data-sts-0",
		"namespace": ns,
	}
	if annotations != nil {
		meta["annotations"] = annotations
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata":   meta,
	}}
}

func newInput(pvc *unstructured.Unstructured, mapping map[string]string) *velero.RestoreItemActionExecuteInput {
	return &velero.RestoreItemActionExecuteInput{
		Item:           pvc,
		ItemFromBackup: pvc,
		Restore: &velerov1api.Restore{
			Spec: velerov1api.RestoreSpec{NamespaceMapping: mapping},
		},
	}
}

func TestAppliesTo(t *testing.T) {
	sel, err := NewRestorePlugin(logrus.New()).AppliesTo()
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.IncludedResources) != 1 || sel.IncludedResources[0] != "persistentvolumeclaims" {
		t.Fatalf("unexpected selector %+v", sel)
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name       string
		annotation interface{}
		mapping    map[string]string
		want       string
		wantErr    bool
	}{
		{
			name:       "namespace mapped",
			annotation: srcAffinityGroup,
			mapping:    map[string]string{"src": "dst"},
			want:       "dst/sts",
		},
		{
			name:       "no mapping keeps backup namespace",
			annotation: srcAffinityGroup,
			want:       srcAffinityGroup,
		},
		{
			name:       "mapping for other namespace is ignored",
			annotation: srcAffinityGroup,
			mapping:    map[string]string{"other": "dst"},
			want:       srcAffinityGroup,
		},
		{
			name: "missing annotation is skipped",
		},
		{
			name:       "invalid annotation",
			annotation: "sts",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ann map[string]interface{}
			if tt.annotation != nil {
				ann = map[string]interface{}{STSAffinityGroupAnnotation: tt.annotation}
			}
			pvc := newPVC("src", ann)

			out, err := NewRestorePlugin(logrus.New()).Execute(newInput(pvc, tt.mapping))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			got := out.UpdatedItem.(*unstructured.Unstructured).GetAnnotations()[STSAffinityGroupAnnotation]
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
