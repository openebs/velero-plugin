/*
Copyright 2020 The OpenEBS Authors.

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

package utils

import (
	"net"
	"time"
	"strconv"
	"strings"
	"github.com/pkg/errors"
	"github.com/gofrs/uuid"
	cloud "github.com/openebs/velero-plugin/pkg/clouduploader"
)

const (
	// IdentifierKey is a word to generate snapshotID from volume name and backup name
	IdentifierKey = "."
	// restored PV prefix name
	RestorePrefix = "restored-"
)

func GetServerAddress() (string, error) {
	netInterfaceAddresses, err := net.InterfaceAddrs()

	if err != nil {
		return "", err
	}

	for _, netInterfaceAddress := range netInterfaceAddresses {
		networkIP, ok := netInterfaceAddress.(*net.IPNet)
		if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
			ip := networkIP.IP.String()
			return ip + ":" + strconv.Itoa(cloud.RecieverPort), nil
		}
	}
	return "", errors.New("error: fetching the interface")
}

func GenerateSnapshotID(volumeID, backupName string) string {
	return volumeID + IdentifierKey + backupName
}

// GetInfoFromSnapshotID return backup name and volume id from the given snapshotID
func GetInfoFromSnapshotID(snapshotID string) (volumeID, backupName string, err error) {
	s := strings.Split(snapshotID, IdentifierKey)
	if len(s) != 2 {
		err = errors.New("invalid snapshot id")
		return
	}

	volumeID = s[0]
	backupName = s[1]

	if volumeID == "" || backupName == "" {
		err = errors.Errorf("invalid volumeID=%s backupName=%s", volumeID, backupName)
	}
	return
}

// GetRestorePVName return new name for clone pv for the given pv
func GetRestorePVName() (string, error) {
	nuuid, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrapf(err, "zfs: error generating uuid for PV rename")
	}

	return RestorePrefix + nuuid.String(), nil
}

// GetScheduleName return the schedule name for the given backup
// It will check if backup name have 'bkp-20060102150405' format
func GetScheduleName(backupName string) string {
	// for non-scheduled backup, we are considering backup name as schedule name only
	schdName := ""

	// If it is scheduled backup then we need to get the schedule name
	splitName := strings.Split(backupName, "-")
	if len(splitName) >= 2 {
		_, err := time.Parse("20060102150405", splitName[len(splitName)-1])
		if err != nil {
			// last substring is not timestamp, so it is not generated from schedule
			return ""
		}
		schdName = strings.Join(splitName[0:len(splitName)-1], "-")
	}
	return schdName
}
