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

package velero

import (
	"fmt"
	"math/rand"
	"time"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	clientset "github.com/heptio/velero/pkg/generated/clientset/versioned"
	config "github.com/openebs/velero-plugin/tests/config"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyz"

	// VeleroNamespace is velero namespace
	VeleroNamespace = "velero"
)

// ClientSet interface for Velero API
type ClientSet struct {
	clientset.Interface
}

var (
	// Client client for Velero API interface
	Client *ClientSet

	// BackupLocation backup location
	BackupLocation string

	// SnapshotLocation snapshot location
	SnapshotLocation string
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func init() {
	cfg, err := config.GetClusterConfig()
	if err != nil {
		panic(err)
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
	Client = &ClientSet{client}
}

func generateRandomString(length int) string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func (c *ClientSet) generateBackupName() (string, error) {
	for i := 0; i < 10; {
		b := generateRandomString(8)
		if len(b) == 0 {
			continue
		}
		_, err := c.VeleroV1().
			Backups(VeleroNamespace).
			Get(b, metav1.GetOptions{})
		if err != nil && k8serrors.IsNotFound(err) {
			return b, nil
		}
	}
	return "", errors.New("Failed to generate unique backup name")
}

// CreateBackup creates the backup for given namespace
func (c *ClientSet) CreateBackup(ns string) (string, v1.BackupPhase, error) {
	var status v1.BackupPhase
	snapVolume := true

	bname, err := c.generateBackupName()
	if err != nil {
		return "", status, err
	}
	bkp := &v1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bname,
			Namespace: VeleroNamespace,
		},
		Spec: v1.BackupSpec{
			IncludedNamespaces:      []string{ns},
			SnapshotVolumes:         &snapVolume,
			StorageLocation:         BackupLocation,
			VolumeSnapshotLocations: []string{SnapshotLocation},
		},
	}
	o, err := c.VeleroV1().
		Backups(VeleroNamespace).
		Create(bkp)
	if err != nil {
		return "", status, err
	}

	if status, err = c.waitForBackupCompletion(o.Name); err == nil {
		return o.Name, status, nil
	}

	return bname, status, err
}

func (c *ClientSet) waitForBackupCompletion(name string) (v1.BackupPhase, error) {
	dumpLog := 0
	for {
		bkp, err := c.getBackup(name)
		if err != nil {
			return "", err
		}
		if isBackupDone(bkp) {
			return bkp.Status.Phase, nil
		}
		if dumpLog > 6 {
			fmt.Printf("Waiting for backup %s completion..\n", name)
			dumpLog = 0
		}
		dumpLog++
		time.Sleep(5 * time.Second)
	}
}

func (c *ClientSet) getBackup(name string) (*v1.Backup, error) {
	return c.VeleroV1().
		Backups(VeleroNamespace).
		Get(name, metav1.GetOptions{})
}
