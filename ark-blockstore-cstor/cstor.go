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
	"net"

        "github.com/sirupsen/logrus"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/client-go/kubernetes"
        "k8s.io/client-go/rest"
        v1alpha1 "github.com/openebs/maya/pkg/apis/openebs.io/v1alpha1"
)

const (
	mayaAPIServiceName = "maya-apiserver-service"
	snapshotCreatePath = "/latest/snapshots/"
	operator = "openebs"
	casType = "cstor"
	backupDir = "backups"
)

type cstorSnap struct {
        Log logrus.FieldLogger
}

// Snapshot keeps track of snapshots created by this plugin
type Snapshot struct {
        volID, backupName, namespace, az string
}

func GetHostIp() string {
        netInterfaceAddresses, err := net.InterfaceAddrs()

        if err != nil { return "" }

        for _, netInterfaceAddress := range netInterfaceAddresses {
                networkIp, ok := netInterfaceAddress.(*net.IPNet)
                if ok && !networkIp.IP.IsLoopback() && networkIp.IP.To4() != nil {
                        ip := networkIp.IP.String()
                        logrus.Infof("Resolved Host IP: " + ip)
                        return ip
                }
        }
        return ""
}

func (p *cstorSnap) getMapiAddr() string {
        // creates the in-cluster config
        conf, err := rest.InClusterConfig()
        if err != nil {
		p.Log.Errorf("Failed to get cluster config", err)
		return ""
        }

        // creates the clientset
	clientset, err := kubernetes.NewForConfig(conf)
        if err != nil {
		p.Log.Errorf("Error creating clientset", err)
		return ""
        }

	sc, err := clientset.CoreV1().Services(operator).Get(mayaAPIServiceName, metav1.GetOptions{})
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

func (p *cstorSnap) snapDeleteReq(snapInfo Snapshot, config map[string]string) error {
	if snapInfo.volID == "" || snapInfo.backupName == "" || snapInfo.namespace == "" {
		return fmt.Errorf("Got insufficient info vol:%s snap:%s ns:%s", snapInfo.volID, snapInfo.backupName, snapInfo.namespace)
	}

	addr := p.getMapiAddr()
        url := addr + snapshotCreatePath + snapInfo.backupName

        req, err := http.NewRequest("DELETE", url, nil)

        p.Log.Infof("Deleting snapshot %s of %s volume %s in namespace %s", snapInfo.backupName, snapInfo.volID, snapInfo.namespace)

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

	clutils := &cloudUtils{Log: p.Log}
	ret := clutils.RemoveSnapshot(snapInfo.volID, snapInfo.backupName, config)
	if ret != false {
		return errors.New("Failed to upload snapshot")
	}

        return nil
}

func (p *cstorSnap) snapCreateReq(volName, snapName, namespace string, config map[string]string) error {
        var snap v1alpha1.CASSnapshot

	addr := p.getMapiAddr()

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
                return fmt.Errorf("Error when connecting maya-apiserver %v", err)
        }
        defer resp.Body.Close()

        _, err = ioutil.ReadAll(resp.Body)
        if err != nil {
                return fmt.Errorf("Unable to read response from maya-apiserver %v", err)
        }

        code := resp.StatusCode
        if code != http.StatusOK {
                return fmt.Errorf("Status error: %v\n", http.StatusText(code))
        }

        p.Log.Infof("Snapshot Successfully Created")

	clutils := &cloudUtils{Log: p.Log}
	ret := clutils.UploadSnapshot(volName, snapName, config)
	if ret != true {
		return errors.New("Failed to upload snapshot")
	} else {
		return nil
	}
}

func (p *cstorSnap) getSnapInfo(snapshotID string) (*Snapshot, error) {
        s := strings.Split(snapshotID, "-ark-bkp-")
        volumeID := s[0]
        bkpName := s[1]

        conf, err := rest.InClusterConfig()
        if err != nil {
		return nil, err
        }

        clientset, err := kubernetes.NewForConfig(conf)
        if err != nil {
		return nil, err
        }

        pvcList, err := clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
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
