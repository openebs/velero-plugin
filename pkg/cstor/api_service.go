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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
		if err = resp.Body.Close(); err != nil {
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
func (p *Plugin) getMapiAddr() (string, error) {
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
		if k8serrors.IsNotFound(err) {
			return "", nil
		}
		p.Log.Errorf("Error getting maya-apiservice : %v", err.Error())
		return "", err
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
		if k8serrors.IsNotFound(err) {
			return "", nil
		}
		p.Log.Errorf("Error getting IP Address for service{%s} : %v", mayaAPIServiceName, err.Error())
		return "", err
	}

fetchip:
	for _, s := range svclist.Items {
		if s.Spec.ClusterIP != "" {
			// update the namespace
			p.namespace = s.Namespace
			return "http://" + s.Spec.ClusterIP + ":" + strconv.FormatInt(int64(s.Spec.Ports[0].Port), 10), nil
		}
	}

	return "", nil
}

// getCVCAddr return CVC server's ip address
func (p *Plugin) getCVCAddr() (string, error) {
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
				LabelSelector: cvcAPIServiceLabel,
			},
		)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return "", nil
		}
		p.Log.Errorf("Error getting cvc service: %v", err.Error())
		return "", err
	}

	if len(svclist.Items) == 0 {
		return "", nil
	}

	for _, s := range svclist.Items {
		if s.Spec.ClusterIP != "" {
			// update the namespace
			p.namespace = s.Namespace
			return "http://" + s.Spec.ClusterIP + ":" + strconv.FormatInt(int64(s.Spec.Ports[0].Port), 10), nil
		}
	}

	return "", nil
}

func (p *Plugin) sendBackupRequest(vol *Volume) (*v1alpha1.CStorBackup, error) {
	var url string

	scheduleName := p.getScheduleName(vol.backupName) // This will be backup/schedule name

	bkpSpec := &v1alpha1.CStorBackupSpec{
		BackupName: scheduleName,
		VolumeName: vol.volname,
		SnapName:   vol.backupName,
		BackupDest: p.cstorServerAddr,
		LocalSnap:  p.local,
	}

	bkp := &v1alpha1.CStorBackup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: vol.namespace,
		},
		Spec: *bkpSpec,
	}

	if vol.isCSIVolume {
		url = p.cvcAddr + backupEndpoint
	} else {
		url = p.mayaAddr + backupEndpoint
	}

	bkpData, err := json.Marshal(bkp)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing json")
	}

	_, err = p.httpRestCall(url, "POST", bkpData)
	if err != nil {
		return nil, errors.Wrapf(err, "Error calling REST api")
	}

	return bkp, nil
}

func (p *Plugin) sendRestoreRequest(vol *Volume) (*v1alpha1.CStorRestore, error) {
	var url string

	restoreSrc := p.cstorServerAddr
	if p.local {
		restoreSrc = vol.srcVolname
	}

	restore := &v1alpha1.CStorRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: p.namespace,
		},
		Spec: v1alpha1.CStorRestoreSpec{
			RestoreName:  vol.backupName,
			VolumeName:   vol.volname,
			RestoreSrc:   restoreSrc,
			StorageClass: vol.storageClass,
			Size:         vol.size,
			Local:        p.local,
		},
	}

	if vol.isCSIVolume {
		url = p.cvcAddr + restorePath
	} else {
		url = p.mayaAddr + restorePath
	}

	restoreData, err := json.Marshal(restore)
	if err != nil {
		return nil, err
	}

	data, err := p.httpRestCall(url, "POST", restoreData)
	if err != nil {
		return nil, errors.Wrapf(err, "Error executing REST api for restore")
	}

	// if apiserver is having version <=1.8 then it will return empty response
	ok, err := isEmptyRestResponse(data)
	if !ok && err == nil {
		// TODO: for CSI base volume response type may be different
		err = p.updateVolCASInfo(data, vol.volname)
		if err != nil {
			err = errors.Wrapf(err, "Error parsing restore API response")
		}
	}

	return restore, err
}

func isEmptyRestResponse(data []byte) (bool, error) {
	var obj interface{}

	dec := json.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&obj)
	if err != nil {
		return false, err
	}

	res, ok := obj.(string)
	if ok && res == "" {
		return true, nil
	}

	return false, nil
}

func (p *Plugin) sendDeleteRequest(backup, volume, namespace, schedule string, isCSIVolume bool) error {
	var url string

	if isCSIVolume {
		url = p.cvcAddr + backupEndpoint + backup
	} else {
		url = p.mayaAddr + backupEndpoint + backup
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create HTTP request")
	}

	q := req.URL.Query()
	q.Add("volume", volume)
	q.Add("namespace", namespace)
	q.Add("schedule", schedule)

	req.URL.RawQuery = q.Encode()

	c := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to connect maya-apiserver")
	}

	defer func() {
		if err = resp.Body.Close(); err != nil {
			p.Log.Warnf("Failed to close response err=%s", err)
		}
	}()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read response from maya-apiserver")
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return errors.Errorf("HTTP Status error{%v} from maya-apiserver", code)
	}

	return nil
}
