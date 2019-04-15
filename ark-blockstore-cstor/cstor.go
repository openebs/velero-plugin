package main

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	v1alpha1 "github.com/payes/maya/pkg/apis/openebs.io/v1alpha1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	mayaAPIServiceName   = "maya-apiserver-service"
	backupCreatePath     = "/latest/backups/"
	restorePath          = "/latest/restore/"
	operator             = "openebs"
	casType              = "cstor"
	backupDir            = "backups"
	backupStatusInterval = 1
)

type cstorSnapPlugin struct {
	Plugin          cloudprovider.BlockStore
	Log             logrus.FieldLogger
	K8sClient       corev1.CoreV1Interface
	config          map[string]string
	cl              *cloudUtils
	mayaAddr        string
	cstorServerAddr string
	volumes         map[string]*Volume
	snapshots       map[string]*Snapshot
}

// Snapshot describes snapshot object information
type Snapshot struct {
	//Volume name on which snapshot should be taken
	volID string

	//backupName is name of a snapshot
	backupName string

	//namespace is volume's namespace
	namespace string
}

// Volume describes volume object information
type Volume struct {
	//volname is volume name
	volname string

	//casType is volume type
	casType string

	//namespace is volume's namespace
	namespace string

	//pvc is name of the PVC object related to given volume
	pvc string

	//backupName is snapshot name for given volume
	backupName string

	//status is backup progress status for given volume
	status v1alpha1.BackupCStorStatus
}

func (p *cstorSnapPlugin) getServerAddress() string {
	netInterfaceAddresses, err := net.InterfaceAddrs()

	if err != nil {
		p.Log.Errorf("Failed to get interface Address for ark server:%v", err)
		return ""
	}

	for _, netInterfaceAddress := range netInterfaceAddresses {
		networkIP, ok := netInterfaceAddress.(*net.IPNet)
		if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
			ip := networkIP.IP.String()
			logrus.Infof("Resolved Host IP: " + ip)
			return ip + ":" + strconv.Itoa(RecieverPort)
		}
	}
	return ""
}

func (p *cstorSnapPlugin) getMapiAddr() string {
	sc, err := p.K8sClient.Services(operator).Get(mayaAPIServiceName, metav1.GetOptions{})
	if err != nil {
		p.Log.Errorf("Error getting IP Address for service - %s : %v", mayaAPIServiceName, err)
		return ""
	}

	if len(sc.Spec.ClusterIP) != 0 {
		return "http://" + sc.Spec.ClusterIP + ":" + strconv.FormatInt(int64(sc.Spec.Ports[0].Port), 10)
	}
	return ""
}

func (p *cstorSnapPlugin) Init(config map[string]string) error {
	conf, err := rest.InClusterConfig()
	if err != nil {
		p.Log.Errorf("Failed to get cluster config", err)
		return errors.New("Error fetching cluster config")
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		p.Log.Errorf("Error creating clientset", err)
		return errors.New("Error creating k8s client")
	}

	p.K8sClient = clientset.CoreV1()
	p.mayaAddr = p.getMapiAddr()
	if p.mayaAddr == "" {
		return errors.New("Error fetching OpenEBS rest client address")
	}

	p.cstorServerAddr = p.getServerAddress()
	if p.cstorServerAddr == "" {
		return errors.New("Error fetch cstorArkServer address")
	}
	p.config = config
	if p.volumes == nil {
		p.volumes = make(map[string]*Volume)
	}
	if p.snapshots == nil {
		p.snapshots = make(map[string]*Snapshot)
	}

	p.cl = &cloudUtils{Log: p.Log}
	return p.cl.InitCloudConn(config)
}

func (p *cstorSnapPlugin) GetVolumeID(pv runtime.Unstructured) (string, error) {
	if !collections.Exists(pv.UnstructuredContent(), "metadata") {
		return "", nil
	}

	// Seed the volume info so that GetVolumeInfo doesn't fail later.
	volumeID, _ := collections.GetString(pv.UnstructuredContent(), "metadata.name")
	if _, exists := p.volumes[volumeID]; !exists {
		sc, _ := collections.GetString(pv.UnstructuredContent(), "spec.storageClassName")
		ns, _ := collections.GetString(pv.UnstructuredContent(), "spec.claimRef.namespace")
		p.volumes[volumeID] = &Volume{
			volname:   volumeID,
			casType:   sc,
			namespace: ns,
		}
	}

	return collections.GetString(pv.UnstructuredContent(), "metadata.name")
}

func (p *cstorSnapPlugin) DeleteSnapshot(snapshotID string) error {
	var snapInfo *Snapshot
	var err error

	p.Log.Infof("Deleting snapshot %v", snapshotID)
	if _, exists := p.snapshots[snapshotID]; !exists {
		snapInfo, err = p.getSnapInfo(snapshotID)
		if err != nil {
			return err
		}
		p.snapshots[snapshotID] = snapInfo
	} else {
		snapInfo = p.snapshots[snapshotID]
	}

	if snapInfo.volID == "" || snapInfo.backupName == "" || snapInfo.namespace == "" {
		return fmt.Errorf("Got insufficient info vol:%s snap:%s ns:%s", snapInfo.volID, snapInfo.backupName, snapInfo.namespace)
	}

	url := p.mayaAddr + backupCreatePath + snapInfo.backupName

	req, err := http.NewRequest("DELETE", url, nil)

	q := req.URL.Query()
	q.Add("volume", snapInfo.volID)
	q.Add("namespace", snapInfo.namespace)
	q.Add("casType", casType)

	req.URL.RawQuery = q.Encode()

	c := &http.Client{
		Timeout: 60 * time.Second,
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("Error when connecting maya-apiserver %v", err)
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Unable to read response from maya-apiserver:%v", err)
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return fmt.Errorf("HTTP Status error from maya-apiserver:%d", code)
	}

	filename := p.generateRemoteFilename(snapInfo.volID, snapInfo.backupName)
	if filename == "" {
		return fmt.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.RemoveSnapshot(filename)
	if ret != false {
		return errors.New("Failed to remove snapshot")
	}

	return nil
}

func (p *cstorSnapPlugin) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	var vol *Volume

	p.cl.exitServer = false
	bkpname, terr := tags["ark.heptio.com/backup"]
	if terr != true {
		return "", errors.New("Failed to get backup name")
	}

	if _, ret := p.volumes[volumeID]; !ret {
		return "", errors.New("Volume is not found")
	}

	vol = p.volumes[volumeID]
	vol.backupName = bkpname
	err := p.backupPVC(volumeID)
	if err != nil {
		return "", errors.New("failed to create backup for PVC")
	}

	p.Log.Infof("creating snapshot %v", bkpname)

	splitName := strings.Split(bkpname, "-")
	bname := ""
	if len(splitName) >= 2 {
		bname = strings.Join(splitName[0:len(splitName)-1], "-")
	} else {
		bname = bkpname
	}

	if len(strings.TrimSpace(bkpname)) == 0 {
		return "", errors.New("no bkpname")
	}

	bkpSpec := &v1alpha1.BackupCStorSpec{
		BackupName: bname,
		VolumeName: volumeID,
		SnapName:   bkpname,
		BackupDest: p.cstorServerAddr,
	}

	bkp := &v1alpha1.BackupCStor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: vol.namespace,
		},
		Spec: *bkpSpec,
	}

	url := p.mayaAddr + backupCreatePath

	bkpData, _ := json.Marshal(bkp)
	_, err = p.httpRestCall(url, "POST", bkpData)
	if err != nil {
		return "", fmt.Errorf("Error calling REST api:%v", err)
	}

	p.Log.Infof("Snapshot Successfully Created")
	filename := p.generateRemoteFilename(volumeID, vol.backupName)
	if filename == "" {
		return "", fmt.Errorf("Error creating remote file name for backup")
	}

	go p.checkBackupStatus(bkp)

	/*
	 * For backup, we are downloading snapshot from one replica only
	 */
	MaxRetryCount = 1
	ret := p.cl.UploadSnapshot(filename)
	if ret != true {
		return "", errors.New("Failed to upload snapshot")
	}

	p.Log.Infof("got status findal :%v", vol.status)
	if vol.status == v1alpha1.BKPCStorStatusDone {
		return volumeID + "-ark-bkp-" + bkpname, nil
	}
	return "", fmt.Errorf("Failed to upload snapshot:%v", vol.status)
}

func (p *cstorSnapPlugin) getSnapInfo(snapshotID string) (*Snapshot, error) {
	s := strings.Split(snapshotID, "-ark-bkp-")
	volumeID := s[0]
	bkpName := s[1]

	pvcList, err := p.K8sClient.PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error fetching namespaces for %s : %v", volumeID, err)
	}

	pvNs := ""
	for _, pvc := range pvcList.Items {
		if volumeID == pvc.Spec.VolumeName {
			pvNs = pvc.Namespace
			break
		}
	}

	if pvNs == "" {
		return nil, errors.New("Failed to find namespace for PVC")
	}
	return &Snapshot{
		volID:      volumeID,
		backupName: bkpName,
		namespace:  pvNs,
	}, nil
}

func (p *cstorSnapPlugin) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
	var restoreResp v1alpha1.RestoreResp

	p.cl.exitServer = false
	if volumeType != "cstor-snapshot" {
		return "", fmt.Errorf("Invalid volume type(%s)", volumeType)
	}

	s := strings.Split(snapshotID, "-ark-bkp-")
	volumeID := s[0]
	snapName := s[1]

	p.Log.Infof("Restoring snapshot %s for volume:%s", snapName, volumeID)

	newVol, e := p.createPVC(volumeID, snapName)
	if e != nil {
		return "", fmt.Errorf("Failed to restore PVC")
	}

	p.Log.Infof("New volume(%v) created", newVol)

	restoreSpec := &v1alpha1.CStorRestoreSpec{
		Name:       newVol.backupName,
		VolumeName: newVol.volname,
		CasType:    newVol.casType,
		RestoreSrc: p.cstorServerAddr,
	}

	restore := &v1alpha1.CStorRestore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: newVol.namespace,
		},
		Spec: *restoreSpec,
	}

	url := p.mayaAddr + restorePath

	restoreData, _ := json.Marshal(restore)
	resp, err := p.httpRestCall(url, "POST", restoreData)
	if err != nil {
		p.Log.Errorf("Error executing REST api : %v", err)
		return "", fmt.Errorf("Error executing REST api for restore : %v", err)
	}

	r := strings.Replace(string(resp), "\"", "", -1)
	dresp, err := b64.StdEncoding.DecodeString(string(r))
	if err != nil {
		p.Log.Errorf("Error decoding respons : %v", err)
		return "", fmt.Errorf("Error failed to decode response : %v", err)
	}

	if err := json.Unmarshal(dresp, &restoreResp); err != nil {
		p.Log.Errorf("Error failed to unmarshal response : %v", err)
		return "", fmt.Errorf("Error failed to unmarshal:%v", err)
	}

	MaxRetryCount = restoreResp.ReplicaCount
	filename := p.generateRemoteFilename(volumeID, snapName)
	if filename == "" {
		p.Log.Errorf("Error failed to create remote file-name for backup")
		return "", fmt.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.RestoreSnapshot(filename)
	if ret != true {
		p.Log.Errorf("Failed to restore snapshot")
		return "", errors.New("Failed to restore snapshot")
	}

	p.Log.Infof("Restore completed..")
	return volumeID, nil
}

func (p *cstorSnapPlugin) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	return "cstor-snapshot", nil, nil
}

func (p *cstorSnapPlugin) createPVC(volumeID, snapName string) (*Volume, error) {
	var pvc v1.PersistentVolumeClaim
	var data []byte
	var ok bool

	filename := p.generateRemoteFilename(volumeID, snapName)
	if filename == "" {
		return nil, errors.New("Error creating remote file name for pvc backup")
	}

	if data, ok = p.cl.ReadFromFile(filename + ".pvc"); !ok {
		return nil, errors.New("Failed to upload PVC")
	}

	if err := json.Unmarshal(data, &pvc); err != nil {
		return nil, errors.New("Failed to decode pvc")
	}

	newVol, err := p.checkIfPVCExist(pvc)
	if err == nil {
		newVol.backupName = snapName
		return newVol, nil
	}

	pvc.Annotations = make(map[string]string)
	pvc.Annotations["openebs.io/created-through"] = "restore"
	rpvc, er := p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Create(&pvc)
	if er != nil {
		return nil, fmt.Errorf("Failed to create PVC err:%v", er)
	}

	for {
		pvc, er := p.K8sClient.PersistentVolumeClaims(rpvc.Namespace).Get(rpvc.Name, metav1.GetOptions{})
		if er != nil || pvc.Status.Phase == v1.ClaimLost {
			p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Delete(rpvc.Name, nil)
			return nil, fmt.Errorf("Failed to create PVC err:%v", er)
		}
		if pvc.Status.Phase == v1.ClaimBound {
			p.Log.Infof("PVC(%v) created..", pvc.Name)
			return &Volume{
				volname:    pvc.Spec.VolumeName,
				namespace:  pvc.Namespace,
				backupName: snapName,
				casType:    *pvc.Spec.StorageClassName,
			}, nil
		}
	}
}

func (p *cstorSnapPlugin) backupPVC(volumeID string) error {
	vol := p.volumes[volumeID]
	var bkpPvc *v1.PersistentVolumeClaim

	pvcs, err := p.K8sClient.PersistentVolumeClaims(vol.namespace).List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorf("Error fetching PVC list : %v", err)
		return errors.New("Failed to fetch PVC list")
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == vol.volname {
			bkpPvc = &pvc
			break
		}
	}

	if bkpPvc == nil {
		p.Log.Errorf("Failed to find PVC for PV : %v", vol.volname)
		return fmt.Errorf("Failed to find PVC for volume:%v", vol.volname)
	}

	bkpPvc.ResourceVersion = ""
	bkpPvc.SelfLink = ""
	bkpPvc.Annotations = nil
	bkpPvc.UID = ""
	bkpPvc.Spec.VolumeName = ""

	data, err := json.MarshalIndent(bkpPvc, "", "\t")
	if err != nil {
		return errors.New("Error doing json parsing")
	}

	filename := p.generateRemoteFilename(vol.volname, vol.backupName)
	if filename == "" {
		return errors.New("Error creating remote file name for pvc backup")
	}

	if ok := p.cl.WriteToFile(data, filename+".pvc"); !ok {
		return errors.New("Failed to upload PVC")
	}

	return nil
}

func (p *cstorSnapPlugin) generateRemoteFilename(filename, bkpname string) string {
	return backupDir + "/" + bkpname + "/" + p.cl.prefix + "-" + filename + "-" + bkpname
}

func (p *cstorSnapPlugin) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	metadataMap, err := collections.GetMap(pv.UnstructuredContent(), "spec.hostPath.path")
	if err != nil {
		return nil, err
	}

	metadataMap["volumeID"] = volumeID
	return pv, nil
}

func (p *cstorSnapPlugin) httpRestCall(url, reqtype string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(reqtype, url, bytes.NewBuffer(data))
	req.Header.Add("Content-Type", "application/json")

	c := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error when connecting maya-apiserver %v", err)
	}
	defer resp.Body.Close()

	respdata, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read response from maya-apiserver %v", err)
	}

	code := resp.StatusCode
	if code != http.StatusOK {
		return nil, fmt.Errorf("Status error: %v", http.StatusText(code))
	}
	return respdata, nil
}

func (p *cstorSnapPlugin) checkIfPVCExist(pvc v1.PersistentVolumeClaim) (*Volume, error) {
	rpvc, err := p.K8sClient.PersistentVolumeClaims(pvc.Namespace).Get(pvc.Name, metav1.GetOptions{})
	if err != nil || pvc.Status.Phase == v1.ClaimLost {
		return nil, fmt.Errorf("PVC:%v does not exist", pvc.Name)
	}
	if rpvc.Status.Phase == v1.ClaimLost {
		p.Log.Infof("PVC:%v is not bound yet!", rpvc.Name)
		panic(fmt.Errorf("PVC:%v is not bound yet", rpvc.Name))
	} else {
		return &Volume{
			volname:   rpvc.Spec.VolumeName,
			namespace: rpvc.Namespace,
			casType:   *rpvc.Spec.StorageClassName,
		}, nil
	}
}

func (p *cstorSnapPlugin) checkBackupStatus(bkp *v1alpha1.BackupCStor) {
	var bkpdone bool
	url := p.mayaAddr + backupCreatePath
	bkpvolume, exists := p.volumes[bkp.Spec.VolumeName]

	if !exists {
		p.Log.Errorf("Failed to fetch volume info for %v", bkp.Spec.VolumeName)
		return
	}

	bkpData, _ := json.Marshal(bkp)
	for bkpdone != true {
		time.Sleep(backupStatusInterval * time.Second)
		var bs v1alpha1.BackupCStor

		resp, err := p.httpRestCall(url, "GET", bkpData)
		if err != nil {
			p.Log.Infof("Failed to fetch backup status:%v", err)
			continue
		}

		err = json.Unmarshal(resp, &bs)
		if err != nil {
			p.Log.Infof("Unmarshal failed", err)
			continue
		}

		bkpvolume.status = bs.Status

		switch bs.Status {
		case v1alpha1.BKPCStorStatusDone, v1alpha1.BKPCStorStatusFailed, v1alpha1.BKPCStorStatusInvalid:
			bkpdone = true
			p.cl.exitServer = true
		}
	}
}
