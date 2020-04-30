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

package velero

import velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

// RestoreByCreationTimestamp sorts a list of Restore by creation timestamp, using their names as a tie breaker.
type RestoreByCreationTimestamp []velerov1api.Restore

func (o RestoreByCreationTimestamp) Len() int      { return len(o) }
func (o RestoreByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o RestoreByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}
