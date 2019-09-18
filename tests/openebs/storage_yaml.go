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

package openebs

const (
	// SPCYaml yaml for SPC CR
	SPCYaml = `apiVersion: openebs.io/v1alpha1
kind: StoragePoolClaim
metadata:
  name: sparse-claim-auto
spec:
  name: sparse-claim-auto
  type: sparse
  maxPools: 1
  minPools: 1
  poolSpec:
    poolType: striped
    cacheFile: /var/openebs/sparse/sparse-claim-auto.cache
    overProvisioning: false
`
	// SCYaml for SC CR
	SCYaml = `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: openebs-cstor-sparse-auto
  annotations:
    openebs.io/cas-type: cstor
    cas.openebs.io/config: |
      - name: StoragePoolClaim
        value: "sparse-claim-auto"
      - name: ReplicaCount
        value: "1"
provisioner: openebs.io/provisioner-iscsi
`
	// PVCYaml for PVC CR
	PVCYaml = `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: cstor-vol1-1r-claim
spec:
  storageClassName: openebs-cstor-sparse-auto
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 4G
`
)
