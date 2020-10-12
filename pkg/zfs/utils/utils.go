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
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
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
			return ip, nil
		}
	}
	return "", errors.New("error: fetching the interface")
}

func GenerateResourceName(volumeID, backupName string) string {
	return volumeID + IdentifierKey + backupName
}

func GenerateSnapshotID(volumeID, schdname, backupName string) string {
	return volumeID + IdentifierKey + schdname + IdentifierKey + backupName
}

// GetInfoFromSnapshotID return backup, schdname and volume id from the given snapshotID
func GetInfoFromSnapshotID(snapshotID string) (volumeID, schdname, backupName string, err error) {
	s := strings.Split(snapshotID, IdentifierKey)

	if len(s) == 2 {
		// backward compatibility, old backups
		volumeID = s[0]
		backupName = s[1]
		// for old backups fetch the schdeule from the bkpname
		schdname = GetScheduleName(backupName)
	} else if len(s) == 3 {
		volumeID = s[0]
		schdname = s[1]
		backupName = s[2]
	} else {
		err = errors.New("invalid snapshot id")
		return
	}

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
			return schdName
		}
		schdName = strings.Join(splitName[0:len(splitName)-1], "-")
	}
	return schdName
}
