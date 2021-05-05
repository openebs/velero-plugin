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

package app

import (
	"context"

	"github.com/ghodss/yaml"
	k8s "github.com/openebs/velero-plugin/tests/k8s"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// PVCName PVC name used by this package
	PVCName string
)

// CreateNamespace create namespace for application
func CreateNamespace(ns string) error {
	_, err := k8s.Client.CoreV1().Namespaces().Get(context.TODO(), ns, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			o := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}
			_, err = k8s.Client.CoreV1().Namespaces().Create(context.TODO(), o, metav1.CreateOptions{})
		}
	}
	return err
}

// DestroyNamespace destory the given namespace
func DestroyNamespace(ns string) error {
	err := k8s.Client.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return k8s.Client.WaitForNamespaceCleanup(ns)
	}
	return nil
}

// DeployApplication deploy application
func DeployApplication(appYaml, ns string) error {
	var p corev1.Pod
	if err := yaml.Unmarshal([]byte(appYaml), &p); err != nil {
		return err
	}
	p.Namespace = ns
	_, err := k8s.Client.CoreV1().Pods(ns).Create(context.TODO(), &p, metav1.CreateOptions{})
	if err != nil {
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}

	// update the PVC name used by this application
	PVCName = p.Spec.Volumes[0].PersistentVolumeClaim.ClaimName

	return k8s.Client.WaitForPod(p.Name, p.Namespace)
}

// DestroyApplication destroy the given application
func DestroyApplication(appYaml, ns string) error {
	var p corev1.Pod
	if err := yaml.Unmarshal([]byte(appYaml), &p); err != nil {
		return err
	}
	err := k8s.Client.
		CoreV1().
		Pods(ns).
		Delete(context.TODO(), p.Name, metav1.DeleteOptions{})

	if !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}
