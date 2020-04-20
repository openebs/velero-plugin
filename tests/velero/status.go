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

import v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

func isBackupDone(bkp *v1.Backup) bool {
	var completed bool

	switch bkp.Status.Phase {
	case v1.BackupPhaseFailedValidation, v1.BackupPhaseCompleted, v1.BackupPhasePartiallyFailed, v1.BackupPhaseFailed:
		completed = true
	}
	return completed
}

func isRestoreDone(rst *v1.Restore) bool {
	var completed bool

	switch rst.Status.Phase {
	case v1.RestorePhaseCompleted, v1.RestorePhaseFailed, v1.RestorePhaseFailedValidation, v1.RestorePhasePartiallyFailed:
		completed = true
	}
	return completed
}
