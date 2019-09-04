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

package cstor

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// httpRestCall execute REST API over HTTP
func (p *Plugin) httpRestCall(url, reqtype string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(reqtype, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, errors.Errorf("Error when connecting to maya-apiserver : %s", err.Error())
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			p.Log.Warnf("Failed to close response : %s", err.Error())
		}
	}()

	respdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("Unable to read response from maya-apiserver : %s", err.Error())
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return nil, errors.Errorf("Status error{%v}", http.StatusText(code))
	}
	return respdata, nil
}

// getMapiAddr return maya API server's ip address
func (p *Plugin) getMapiAddr() string {
	var openebsNs string

	// check if user has provided openebs namespace
	if p.namespace != "" {
		openebsNs = p.namespace
	} else {
		openebsNs = metav1.NamespaceAll
	}

	svclist, err := p.K8sClient.
		CoreV1().
		Services(openebsNs).
		List(
			metav1.ListOptions{
				LabelSelector: mayaAPIServiceLabel,
			},
		)

	if err != nil {
		p.Log.Errorf("Error getting maya-apiservice : %v", err.Error())
		return ""
	}

	if len(svclist.Items) != 0 {
		goto fetchip
	}

	// There are no any services having MayaApiService Label
	// Let's try to find by name only..
	svclist, err = p.K8sClient.
		CoreV1().
		Services(openebsNs).
		List(
			metav1.ListOptions{
				FieldSelector: "metadata.name=" + mayaAPIServiceName,
			})
	if err != nil {
		p.Log.Errorf("Error getting IP Address for service{%s} : %v", mayaAPIServiceName, err.Error())
		return ""
	}

fetchip:
	for _, s := range svclist.Items {
		if len(s.Spec.ClusterIP) != 0 {
			// update the namespace
			p.namespace = s.Namespace
			return "http://" + s.Spec.ClusterIP + ":" + strconv.FormatInt(int64(s.Spec.Ports[0].Port), 10)
		}
	}

	return ""
}
