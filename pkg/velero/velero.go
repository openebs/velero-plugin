/*
Copyright 2020 The OpenEBS Authors.

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

	return err
}
