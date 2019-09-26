/*
Copyright 2019 The OpenEBS Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openebs

import (
	"time"

	"github.com/ghodss/yaml"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	clientset "github.com/openebs/maya/pkg/client/generated/clientset/versioned"
	config "github.com/openebs/velero-plugin/tests/config"
	k8s "github.com/openebs/velero-plugin/tests/k8s"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// ClientSet interface for OpenEBS API
type ClientSet struct {
	clientset.Interface
}

var (
	// Client for openebs ClientSet
	Client *ClientSet

	// SPCName for SPC
	SPCName string

	// PVCName for PVC
	PVCName string

	// AppPVC created by openebs
	AppPVC *corev1.PersistentVolumeClaim
)

const (
	// OpenEBSNs openebs Namespace
	OpenEBSNs = "openebs"

	// PVCDeploymentLabel for target pod Deployment
	PVCDeploymentLabel = "openebs.io/persistent-volume-claim"
)

func init() {
	cfg, err := config.GetClusterConfig()
	if err != nil {
		panic(err)
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	Client = &ClientSet{client}
}

// CreateSPC create SPC for given YAML
func (c *ClientSet) CreateSPC(spcYAML string) error {
	var spc v1alpha1.StoragePoolClaim

	if err := yaml.Unmarshal([]byte(spcYAML), &spc); err != nil {
		return err
	}

	_, err := c.OpenebsV1alpha1().StoragePoolClaims().Create(&spc)
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
		SPCName = spc.Name
		return nil
	}

	SPCName = spc.Name

	return k8s.Client.WaitForDeployment(
		string(v1alpha1.StoragePoolClaimCPK)+"="+spc.Name,
		OpenEBSNs)
}

// CreateVolume create volume from given PVC yaml
func (c *ClientSet) CreateVolume(pvcYAML, pvcNs string, wait bool) error {
	var pvc corev1.PersistentVolumeClaim
	var err error

	if err = yaml.Unmarshal([]byte(pvcYAML), &pvc); err != nil {
		return err
	}
	pvc.Namespace = pvcNs

	if err = k8s.Client.CreatePVC(pvc); err != nil {
		return err
	}

	time.Sleep(5 * time.Second)
	PVCName = pvc.Name
	if wait {
		err = c.WaitForHealthyCVR(&pvc)
	}
	if err != nil {
		AppPVC = &pvc
	}
	return err
}

// DeleteVolume delete volume for given PVC YAML
func (c *ClientSet) DeleteVolume(pvcYAML, pvcNs string) error {
	var pvc corev1.PersistentVolumeClaim
	if err := yaml.Unmarshal([]byte(pvcYAML), &pvc); err != nil {
		return err
	}

	pvc.Namespace = pvcNs
	if err := k8s.Client.DeletePVC(pvc); err != nil {
		return err
	}

	return k8s.Client.WaitForDeploymentCleanup(
		PVCDeploymentLabel+"="+pvc.Name,
		OpenEBSNs)
}
