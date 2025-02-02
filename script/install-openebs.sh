#!/bin/bash

# Copyright 2019 The OpenEBS Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if [ -z $VELERO_RELEASE ] || [ -z $OPENEBS_RELEASE ]; then
    exit
fi

echo "Installing iscsi packages"
sudo apt-get install --yes -qq open-iscsi
sudo service iscsid start
sudo systemctl status iscsid  --no-pager

OPENEBS_NAMESPACE="openebs"
helm repo add openebs https://openebs.github.io/openebs
helm repo update
helm install openebs --namespace $OPENEBS_NAMESPACE  openebs/openebs --create-namespace --set engines.replicated.mayastor.enabled=false  --set engines.local.lvm.enabled=false --set zfs-localpv.analytics.enabled=false

echo "Installation complete"

function waitForDeployment() {
	DEPLOY=$1
	NS=$2
	CREATE=true

	if [ $# -eq 3 ] && ! $3 ; then
		CREATE=false
	fi

	for i in $(seq 1 50) ; do
		kubectl get deployment -n ${NS} ${DEPLOY}
		kstat=$?
		if [ $kstat -ne 0 ] && ! $CREATE ; then
			return
		elif [ $kstat -eq 0 ] && ! $CREATE; then
			sleep 3
			continue
		fi

		replicas=$(kubectl get deployment -n ${NS} ${DEPLOY} -o json | jq ".status.readyReplicas")
		if [ "$replicas" == "1" ]; then
			break
		else
			echo "Waiting for ${DEPLOY} to be ready"
			if [ ${DEPLOY} != "maya-apiserver" ] && [ ${DEPLOY} != "openebs-provisioner" ]; then
				dumpMayaAPIServerLogs 10
			fi
			sleep 10
		fi
	done
}

function dumpMayaAPIServerLogs() {
  LC=$1
  MAPIPOD=$(kubectl get pods -o jsonpath='{.items[?(@.spec.containers[0].name=="maya-apiserver")].metadata.name}' -n openebs)
  kubectl logs --tail=${LC} $MAPIPOD -n openebs
  printf "\n\n"
}

waitForDeployment openebs-zfs-localpv-controller openebs

kubectl get pods --all-namespaces
