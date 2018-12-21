package main

import (
	"errors"
        "bytes"
        "fmt"
	"strings"
        "io/ioutil"
        "net/http"
	"encoding/json"
	"time"
	"strconv"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
)

// Volume keeps track of volumes created by this plugin
type Volume struct {
	volType, az, namespace, pvc string
}

// Snapshot keeps track of snapshots created by this plugin
type Snapshot struct {
	volID, backupName, namespace, az string
}

// Plugin for containing state for the blockstore plugin
type cstorSnap struct {
	config map[string]string
	logrus.FieldLogger
	volumes   map[string]Volume
	snapshots map[string]Snapshot
}

var _ cloudprovider.BlockStore = (*cstorSnap)(nil)

const (
	mayaAPIServiceName = "maya-apiserver-service"
	snapshotCreatePath = "/latest/snapshots/"
	operator = "openebs"
	casType = "cstor"
)

func (p *cstorSnap) getMapiAddr() string {
	// creates the in-cluster config
	conf, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		panic(err.Error())
	}

	sc, err := clientset.CoreV1().Services(operator).Get(mayaAPIServiceName, metav1.GetOptions{})
	if err != nil {
		p.Errorf("Error getting IP Address for service - %s : %v", mayaAPIServiceName, err)
		return ""
	}

	if len(sc.Spec.ClusterIP) != 0 {
		return "http://" + sc.Spec.ClusterIP + ":" + strconv.FormatInt(int64(sc.Spec.Ports[0].Port), 10)
	}

	return ""
}

func (p *cstorSnap) snapDeleteReq(addr, volName, snapName, namespace string) (string, error) {
        url := addr + snapshotCreatePath + snapName

	req, err := http.NewRequest("DELETE", url, nil)

        p.Infof("Deleting snapshot %s of %s volume %s in namespace %s", snapName, volName, namespace)

	// Add query params
        q := req.URL.Query()
        q.Add("volume", volName)
        q.Add("namespace", namespace)
        q.Add("casType", casType)

        // Add query params to req
        req.URL.RawQuery = q.Encode()

        c := &http.Client{
                Timeout: 60 * time.Second,
        }
        resp, err := c.Do(req)
        if err != nil {
                p.Errorf("Error when connecting maya-apiserver %v", err)
                return "Could not connect to maya-apiserver", err
        }
        defer resp.Body.Close()

        data, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                p.Errorf("Unable to read response from maya-apiserver %v", err)
                return "Unable to read response from maya-apiserver", err
        }

        code := resp.StatusCode
        if err == nil && code != http.StatusOK {
                return "HTTP Status error from maya-apiserver", fmt.Errorf(string(data))
        }
        if code != http.StatusOK {
                p.Errorf("Status error: %v\n", http.StatusText(code))
                return "HTTP Status error from maya-apiserver", err
        }
        return string(data), nil
}

func (p *cstorSnap) snapCreateReq(addr, volName, snapName, namespace string) (string, error) {
	var snap v1alpha1.CASSnapshot

	snap.Namespace = namespace
	snap.Name = snapName
	snap.Spec.CasType = casType
	snap.Spec.VolumeName = volName

	url := addr + snapshotCreatePath

	snapBytes, _ := json.Marshal(snap)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(snapBytes))
	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		p.Errorf("Error when connecting maya-apiserver %v", err)
		return "Could not connect to maya-apiserver", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		p.Errorf("Unable to read response from maya-apiserver %v", err)
		return "Unable to read response from maya-apiserver", err
	}

	code := resp.StatusCode
	if err == nil && code != http.StatusOK {
		return "HTTP Status error from maya-apiserver", fmt.Errorf(string(data))
	}
	if code != http.StatusOK {
		p.Errorf("Status error: %v\n", http.StatusText(code))
		return "HTTP Status error from maya-apiserver", err
	}

	p.Infof("Snapshot Successfully Created")
	return "Snapshot Successfully Created", nil
}

// Init the plugin
func (p *cstorSnap) Init(config map[string]string) error {
	p.Infof("Initializing ark plugin for cstor")
	p.config = config

	// Make sure we don't overwrite data, now that we can re-initialize the plugin
	if p.volumes == nil {
		p.volumes = make(map[string]Volume)
	}
	if p.snapshots == nil {
		p.snapshots = make(map[string]Snapshot)
	}

	return nil
}

// CreateVolumeFromSnapshot Create a volume form given snapshot
func (p *cstorSnap) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	p.Infof("Creating volume from snapshot", snapshotID, volumeType, volumeAZ, *iops)
	return snapshotID, nil
}

// GetVolumeInfo Get information about the volume
func (p *cstorSnap) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return "cstor-snapshot", nil, nil
}

// IsVolumeReady Check if the volume is ready.
func (p *cstorSnap) IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error) {
	return true, nil
}

// CreateSnapshot Create a snapshot
func (p *cstorSnap) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	snapshotID := ""

	ns := p.volumes[volumeID].namespace

	bkpname, terr := tags["ark.heptio.com/backup"]
        if terr != true {
                return "",  errors.New("Failed to get backup name")
        }

	mapiAddr := p.getMapiAddr()
	if len(mapiAddr) == 0 {
		return "", errors.New("Failed to get service address")
	}

	p.Infof("Sending snap create request")
	_, err := p.snapCreateReq(mapiAddr, volumeID, bkpname, ns)
	if err == nil {
		snapshotID = volumeID + "-ark-bkp-" + bkpname
	}

	return snapshotID, err
}

func (p *cstorSnap) getSnapInfo(snapshotID string) (string, error) {
	s := strings.Split(snapshotID, "-ark-bkp-")
	volumeID := s[0]
	bkpName := s[1]

	// creates the in-cluster config
	conf, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		panic(err.Error())
	}

	pvcList, err := clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		p.Errorf("Error fetching namespaces for %s : %v", volumeID, err)
		return "", err
	}

	pvNs := ""
	for _, pvc := range pvcList.Items {
		if volumeID == pvc.Spec.VolumeName {
			p.Infof("got pvc namespaces %s", pvc.Namespace)
			pvNs = pvc.Namespace
			break
		}
	}

	if pvNs == "" {
		return "", errors.New("Failed to find PVC")
	}

	if _, exists := p.snapshots[snapshotID]; !exists {
		p.snapshots[snapshotID] = Snapshot {
			volID: volumeID,
			backupName: bkpName,
			namespace: pvNs,
		}
	}

	return snapshotID, nil
}

// DeleteSnapshot Delete a snapshot
func (p *cstorSnap) DeleteSnapshot(snapshotID string) error {
	p.Infof("Deleting snapshot", snapshotID)

	snapID, err := p.getSnapInfo(snapshotID)
	if err != nil {
		p.Errorf("Failed to find snapID %v", err)
		return nil
	}

	snapInfo := p.snapshots[snapID]

	mapiAddr := p.getMapiAddr()
	if len(mapiAddr) == 0 {
		return errors.New("Failed to get service address")
	}

	_, err = p.snapDeleteReq(mapiAddr, snapInfo.volID, snapInfo.backupName, snapInfo.namespace)
	if err == nil {
		snapshotID = snapInfo.volID + "-ark-bkp-" + snapInfo.backupName
	}

	return nil
}

// GetVolumeID Get the volume ID from the spec
func (p *cstorSnap) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "metadata") {
		return "", nil
	}

	// Seed the volume info so that GetVolumeInfo doesn't fail later.
	volumeID, _ := collections.GetString(pv.UnstructuredContent(), "metadata.name")
	if _, exists := p.volumes[volumeID]; !exists {
		sc, _ := collections.GetString(pv.UnstructuredContent(), "spec.storageClassName")
		ns, _ := collections.GetString(pv.UnstructuredContent(), "spec.claimRef.namespace")
		p.volumes[volumeID] = Volume{
			volType: sc,
			namespace: ns,
			pvc: volumeID,
		}
	}

	return collections.GetString(pv.UnstructuredContent(), "metadata.name")
}

// SetVolumeID Set the volume ID in the spec
func (p *cstorSnap) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	metadataMap, err := collections.GetMap(pv.UnstructuredContent(), "spec.hostPath.path")
	if err != nil {
		return nil, err
	}

	metadataMap["volumeID"] = volumeID
	return pv, nil
}
