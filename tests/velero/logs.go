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
	"os"
	"time"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	log "github.com/heptio/velero/pkg/cmd/util/downloadrequest"
)

// DumpBackupLogs dump logs of given backup on stdout
func (c *ClientSet) DumpBackupLogs(backupName string) error {
	backupName = "kasnsssw"

	return log.Stream(c.VeleroV1(),
		VeleroNamespace,
		backupName,
		v1.DownloadTargetKindBackupLog,
		os.Stdout,
		time.Minute)
}
