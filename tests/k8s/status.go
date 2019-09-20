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

package k8s

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPVCPhase return given PVC's phase
func (k *KubeClient) GetPVCPhase(pvc, ns string) (v1.PersistentVolumeClaimPhase, error) {
	o, err := k.CoreV1().
		PersistentVolumeClaims(ns).
		Get(pvc, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return o.Status.Phase, nil
}

func (k *KubeClient) waitForPVCBound(pvc, ns string) (v1.PersistentVolumeClaimPhase, error) {
	for {
		phase, err := k.GetPVCPhase(pvc, ns)
		if err != nil || phase == v1.ClaimLost {
			return phase, errors.Errorf("PVC:%s/%s is in Lost state", ns, pvc)
		}
		if phase == v1.ClaimBound {
			return phase, nil
		}
		time.Sleep(5 * time.Second)
	}

}

// WaitForDeployment wait for deployment having given labelSelector and namespace to be ready
func (k *KubeClient) WaitForDeployment(labelSelector, ns string) error {
	var ready bool
	dumpLog := 0
	for {
		deploymentList, err := k.ExtensionsV1beta1().
			Deployments(ns).
			List(metav1.ListOptions{
				LabelSelector: labelSelector,
			})
		if err != nil {
			return err
		} else if len(deploymentList.Items) == 0 {
			fmt.Printf("Deployment for %s/%s is not availabel..\n", ns, labelSelector)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, d := range deploymentList.Items {
			o, err := k.ExtensionsV1beta1().
				Deployments(d.Namespace).
				Get(d.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if *o.Spec.Replicas == o.Status.UpdatedReplicas {
				ready = true
			} else {
				ready = false
				break
			}
		}

		if ready {
			return nil
		}
		if dumpLog > 6 {
			fmt.Printf("Waiting for deployment for %s/%s to be ready..\n", ns, labelSelector)
			dumpLog = 0
		}
		dumpLog++
		time.Sleep(5 * time.Second)
	}
}

// WaitForPod wait for given pod to become ready
func (k *KubeClient) WaitForPod(podName, podNamespace string) error {
	dumpLog := 0
	for {
		o, err := k.CoreV1().Pods(podNamespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if o.Status.Phase == corev1.PodRunning {
			return nil
		}
		time.Sleep(5 * time.Second)
		if dumpLog > 6 {
			fmt.Printf("checking for pod %s/%s\n", podNamespace, podName)
			dumpLog = 0
		}
		dumpLog++
	}
}

// WaitForDeploymentCleanup wait for cleanup of deployment having given labelSelector and namespace
func (k *KubeClient) WaitForDeploymentCleanup(labelSelector, ns string) error {
	dumpLog := 0
	for {
		deploymentList, err := k.ExtensionsV1beta1().
			Deployments(ns).
			List(metav1.ListOptions{
				LabelSelector: labelSelector,
			})
		if err != nil {
			return err
		} else if len(deploymentList.Items) == 0 {
			return nil
		}
		if dumpLog > 6 {
			fmt.Printf("Waiting for cleanup of deployment %s/%s..\n", ns, labelSelector)
			dumpLog = 0
		}
		dumpLog++
		time.Sleep(5 * time.Second)
	}

}

// GetPodList return list of pod for given label and namespace
func (k *KubeClient) GetPodList(ns, label string) (*corev1.PodList, error) {
	return k.CoreV1().Pods(ns).List(metav1.ListOptions{
		LabelSelector: label,
	})
}
