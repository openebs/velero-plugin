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
	"io"
	"os"
)

// DumpLogs will dump log for given ns/pod
func (k *KubeClient) DumpLogs(ns, podName, container string) error {
	fmt.Printf("################################################\n")
	fmt.Printf("Logs of %s/%s:%s\n", ns, podName, container)
	fmt.Printf("################################################\n")
	req := k.CoreV1().RESTClient().Get().
		Namespace(ns).
		Name(podName).
		Resource("pods").
		SubResource("log").
		Param("container", container).
		//Param("previous", "true").
		Param("timestamps", "true")

	readCloser, err := req.Stream()
	if err != nil {
		fmt.Printf("DumpLogs: Error occurred for %s/%s:%s.. %s", ns, podName, container, err)
		return err
	}

	defer func() {
		_ = readCloser.Close()
	}()

	_, err = io.Copy(os.Stdout, readCloser)
	fmt.Println(err)
	return err
}
