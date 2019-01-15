package main

import (
        "errors"
        "bytes"
        "fmt"
        "strings"
        "io/ioutil"
        "net/http"
//        "encoding/json"
        "time"
        "strconv"
	"net"
	"encoding/json"

	"github.com/heptio/ark/pkg/cloudprovider"
        "github.com/sirupsen/logrus"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/heptio/ark/pkg/util/collections"
	v1 "k8s.io/api/core/v1"
        v1alpha1 "github.com/payes/maya/pkg/apis/openebs.io/v1alpha1"
)

const (
	mayaAPIServiceName = "maya-apiserver-service"
	backupCreatePath = "/latest/backups/"
	operator = "openebs"
	casType = "cstor"
	backupDir = "backups"
)

type cstorSnapPlugin struct {
	Plugin cloudprovider.BlockStore
        Log logrus.FieldLogger
	K8sClient corev1.CoreV1Interface
	config map[string]string
	cl *cloudUtils
	mayaAddr string
	cstorServerAddr string
        volumes   map[string]*Volume
        snapshots map[string]*Snapshot
}

type Snapshot struct {
        volID, backupName, namespace string
}

type Volume struct {
        volname, casType, namespace, pvc, backupName string
}

func (p *cstorSnapPlugin) getServerAddress() string {
        netInterfaceAddresses, err := net.InterfaceAddrs()

        if err != nil {
		p.Log.Errorf("Failed to get interface Address for ark server:%v", err)
		return ""
	}

        for _, netInterfaceAddress := range netInterfaceAddresses {
                networkIp, ok := netInterfaceAddress.(*net.IPNet)
                if ok && !networkIp.IP.IsLoopback() && networkIp.IP.To4() != nil {
                        ip := networkIp.IP.String()
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
        } else {
		return ""
	}
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

	p.cl = &cloudUtils {Log: p.Log}
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
			volname: volumeID,
                        casType: sc,
                        namespace: ns,
                }
        }

        return collections.GetString(pv.UnstructuredContent(), "metadata.name")
}

func (p *cstorSnapPlugin) DeleteSnapshot(snapshotID string) error {
        var snapInfo *Snapshot
        var err error

        p.Log.Infof("Deleting snapshot", snapshotID)
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
		return errors.New("Failed to upload snapshot")
	}

        return nil
}

func (p *cstorSnapPlugin) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (string, error) {
	var vol *Volume

	bkpname, terr := tags["ark.heptio.com/backup"]
	if terr != true {
		return "",  errors.New("Failed to get backup name")
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

	bkpSpec := &v1alpha1.CStorBackupSpec{
		Name: bkpname,
		VolumeName: volumeID,
		CasType: casType,
		BackupDest: p.cstorServerAddr,
	}

	bkp := &v1alpha1.CStorBackup {
		ObjectMeta: metav1.ObjectMeta {
			Namespace: vol.namespace,
		},
		Spec: *bkpSpec,
	}

        url := p.mayaAddr + backupCreatePath

        bkpData, _ := json.Marshal(bkp)
        _, err = p.httpRestCall(url, bkpData)
        if err != nil {
                return "", fmt.Errorf("Error calling REST api:%v", err)
        }

	p.Log.Infof("Snapshot Successfully Created")
	filename := p.generateRemoteFilename(volumeID, vol.backupName)
	if filename == "" {
		return "", fmt.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.UploadSnapshot(filename)
	if ret != true {
		return "", errors.New("Failed to upload snapshot")
	}

	return volumeID + "-ark-bkp-" + bkpname, nil
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
        } else {
		return &Snapshot{
			volID : volumeID,
			backupName : bkpName,
			namespace : pvNs,
		}, nil
	}
}

func (p *cstorSnapPlugin) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (string, error) {
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
		Name: newVol.backupName,
		VolumeName: newVol.volname,
		CasType: newVol.casType,
		RestoreSrc: p.cstorServerAddr,
	}

	restore := &v1alpha1.CStorRestore {
		ObjectMeta: metav1.ObjectMeta {
			Namespace: newVol.namespace,
		},
		Spec: *restoreSpec,
	}

        url := p.mayaAddr + backupCreatePath

        restoreData, _ := json.Marshal(restore)
	_, err := p.httpRestCall(url, restoreData)
	if err != nil {
		return "", fmt.Errorf("Error calling REST api:%v", err)
	}

	filename := p.generateRemoteFilename(volumeID, snapName)
	if filename == "" {
		return "", fmt.Errorf("Error creating remote file name for backup")
	}

	ret := p.cl.RestoreSnapshot(filename)
	if ret != true {
		return "", errors.New("Failed to restore snapshot")
	} else {
		return volumeID, nil
	}

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
			return &Volume {
				volname: pvc.Spec.VolumeName,
				namespace: pvc.Namespace,
				backupName: snapName,
				casType: *pvc.Spec.StorageClassName,
			}, nil
		}
	}
}

func (p *cstorSnapPlugin) backupPVC(volumeID string) error {
	var vol *Volume = p.volumes[volumeID]
	var bkpPvc *v1.PersistentVolumeClaim

	pvcs, err := p.K8sClient.PersistentVolumeClaims(vol.namespace).List(metav1.ListOptions{})
	if err != nil {
		p.Log.Errorf("Error fetching PVCS %v", err)
		return errors.New("Failed to fetch PVC list")
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.VolumeName == vol.volname {
			bkpPvc = &pvc
			break
		}
	}

	if bkpPvc == nil {
		p.Log.Errorf("Failed to find PVC for PV:%v", vol.volname)
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

	if ok := p.cl.WriteToFile(data, filename + ".pvc"); !ok {
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

func (p *cstorSnapPlugin) httpRestCall(url string, data []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
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
		return nil, fmt.Errorf("Status error: %v\n", http.StatusText(code))
	}
	return respdata, nil
}
