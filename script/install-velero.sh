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

export KUBERNETES_SERVICE_HOST="127.0.0.1"
export KUBECONFIG=$HOME/.kube/config

wget -nv -O velero.tar.gz https://github.com/vmware-tanzu/velero/releases/download/${VELERO_RELEASE}/velero-${VELERO_RELEASE}-linux-amd64.tar.gz
mkdir velero
tar xf velero.tar.gz -C velero
velero=$PWD/velero/velero-${VELERO_RELEASE}-linux-amd64/velero
if [ ! -f ${velero} ]; then
    echo "${velero} file does not exist"
    exit
fi

if [ ! -f minio ]; then
	wget -nv https://dl.min.io/server/minio/release/linux-amd64/minio
fi

if [ ! -f mc ]; then
	wget -nv https://dl.min.io/client/mc/release/linux-amd64/mc
fi
chmod +x minio
chmod +x mc

BACKUP_DIR=$PWD/data

export MINIO_ACCESS_KEY=minio
export MINIO_SECRET_KEY=minio123

ip addr show docker0 >> /dev/null
if [ $? -ne 0 ]; then
	exit 1
fi

MINIO_SERVER_IP=`ip addr show docker0 |grep docker0 |grep inet |awk -F ' ' '{print $2}' |awk -F '/' '{print $1}'`
if [ $? -ne 0 ] || [ -z ${MINIO_SERVER_IP} ]; then
	exit 1
fi

./minio server --address ${MINIO_SERVER_IP}:9000 ${BACKUP_DIR} &
MINIO_PID=$!
sleep 5

BUCKET=velero
REGION=minio
./mc config host add velero  http://${MINIO_SERVER_IP}:9000  minio minio123
./mc mb -p velero/velero

${velero} install \
    --provider aws \
    --bucket $BUCKET \
    --secret-file ./script/minio-credentials \
    --backup-location-config region=${REGION},s3ForcePathStyle="true",s3Url=http://${MINIO_SERVER_IP}:9000 \
    --plugins velero/velero-plugin-for-aws-amd64:v1.2.0 \
    --wait

sed "s/MINIO_ENDPOINT/http:\/\/$MINIO_SERVER_IP\:9000/" script/volumesnapshotlocation.yaml > /tmp/s.yaml
kubectl apply -f /tmp/s.yaml
${velero} plugin add openebs/velero-plugin-amd64:ci
