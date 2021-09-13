# Velero-plugin for OpenEBS CStor volume

Velero is a utility to back up and restore your Kubernetes resource and persistent volumes.

To do backup/restore of OpenEBS CStor volumes through Velero utility, you need to install and configure
OpenEBS velero-plugin.

[![Build Status](https://github.com/openebs/velero-plugin/actions/workflows/build.yml/badge.svg)](https://github.com/openebs/velero-plugin/actions/workflows/build.yml)
[![Slack](https://img.shields.io/badge/chat!!!-slack-ff1493.svg?style=flat-square)](https://kubernetes.slack.com/messages/openebs)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_shield)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3900/badge)](https://bestpractices.coreinfrastructure.org/projects/3900)
[![Releases](https://img.shields.io/github/v/release/openebs/velero-plugin.svg?include_prereleases&style=flat-square)](https://github.com/openebs/velero-plugin/releases)
[![LICENSE](https://img.shields.io/github/license/openebs/velero-plugin.svg?style=flat-square)](https://github.com/openebs/velero-plugin/blob/HEAD/LICENSE)

## Table of Contents
- [Compatibility matrix](#compatibility-matrix)
- [Prerequisite for velero-plugin](#prerequisite-for-velero-plugin)
- [Installation of velero-plugin](#installation-of-velero-plugin)
- [Developer Guide](#developer-guide)
- [Local Backup/Restore](#local-backuprestore)
  - [Configuring snapshot location](#configuring-snapshot-location)
  - [Creating a backup](#creating-a-backup)
    - [Creating a restore](#creating-a-restore)
  - [Creating a scheduled backup](#creating-a-scheduled-backup)
    - [Creating a restore from scheduled backup](#creating-a-restore-from-scheduled-backup)
- [Remote Backup/Restore](#remote-backuprestore)
  - [Configuring snapshot location](#configuring-snapshot-location-for-remote-backup)
  - [Creating a backup](#creating-a-remote-backup)
    - [Creating a restore](#creating-a-restore-for-remote-backup)
  - [Creating a scheduled backup](#creating-a-scheduled-remote-backup)
    - [Creating a restore from scheduled backup](#creating-a-restore-from-scheduled-remote-backup)

## Compatibility matrix

|	Velero-plugin Version	|	OpenEBS/Maya Release	|	Velero Version	  |	Codebase	          |
|	--------------------	|	---------------	      |	------------------|	------------------- |
|	0.9.0               	|	>= 0.9                |	>= v0.11.0       	|	v0.9.x	            |
|	1.0.0-velero_1.0.0	  |	>= 1.0.0              |	>= v1.0.0        	|	1.0.0-velero_1.0.0	|
|	1.1.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.1.x	            |
|	1.2.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.2.x            	|
|	1.3.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.3.x	            |
|	1.4.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.4.x	            |
|	1.5.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.5.x	            |
|	1.6.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.6.x	            |
|	1.7.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.7.x           	  |
|	1.8.0-velero_1.0.0  	|	>= 1.0.0              |	>= v1.0.0        	|	v1.8.x	            |
|	1.9.0                	|	>= 1.0.0             	|	>= v1.0.0        	|	v1.9.x	            |
|	1.10.0                |	>= 1.0.0             	|	>= v1.0.0        	|	v1.10.x	            |
|	1.11.0                |	>= 1.0.0             	|	>= v1.0.0        	|	v1.11.x	            |

*Note:*

_OpenEBS version **< 0.9** is not supported for velero-plugin._

_Velero-plugin version **< 1.11.0** is not supported for cstor v1 volumes._

_If you want to use plugin image from development branch(`develop`), use **ci** tag._

Multiarch (amd64/arm64) plugin images are available at [Docker Hub](https://hub.docker.com/r/openebs/velero-plugin/tags).

## Prerequisite for velero-plugin
A Specific version of Velero needs to be installed as per the [compatibility matrix](#Compatibility-matrix) with OpenEBS versions.

For installation steps of Velero, visit https://velero.io.

For installation steps of OpenEBS, visit https://github.com/openebs/openebs/releases.

## Installation of velero-plugin
Run the following command to install development image of OpenEBS velero-plugin

`velero plugin add openebs/velero-plugin:1.9.0`

This command will add an init container to Velero deployment to install the OpenEBS velero-plugin.

## Developer Guide
#### To build the plugin binary
```
make build
```

#### To build the docker image for velero-plugin
```
make container IMAGE=<REPO NAME>
```

#### To push the image to repo
```
make deploy-image IMAGE=<REPO NAME>
```

## Local Backup/Restore
For Local Backup Velero-plugin creates a snapshot for CStor Volume.

### Configuring snapshot location
To take local backup of cStor volume, configure VolumeSnapshotLocation with provider `openebs.io/cstor-blockstore` and set `local` to `true`.
Sample YAML file for volumesnapshotlocation can be found at `example/06-local-volumesnapshotlocation.yaml`.

Sample Spec for volumesnapshotlocation:

```yaml
spec:
  provider: openebs.io/cstor-blockstore
  config:
    namespace: <OPENEBS_NAMESPACE>
    local: "true"
```

If you have multiple installation of openebs then you need to add `spec.config.namespace: <OPENEBS_NAMESPACE>`.

### Creating a backup
Once the volumesnapshotlocation is configured, you can create a backup of your CStor persistent storage volume.

To back up data of all your applications in the default namespace, run the following command:

```
velero backup create localbackup --include-namespaces=default --snapshot-volumes --volume-snapshot-locations=<SNAPSHOT_LOCATION>
```

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-local-volumesnapshotlocation.yaml`.

You can check the status of backup using the following command:

`velero backup get `

Above command will list out the all backups you created. Sample output of the above command is mentioned below :
```
NAME                STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
localbackup         Completed   2019-05-09 17:08:41 +0530 IST   26d       gcp                <none>
```
Once the backup is completed you should see the backup marked as `Completed`.

#### Creating a restore
To restore local backup, run the following command:

```
velero restore create --from-backup backup_name --restore-volumes=true --namespace-mappings source_ns:destination_ns
```

*Note:*
- _Restore from local backup can be done in same cluster, and in different namespace, only where local backups are created_

*Limitation:*
- _Restore of PV having storageClass, with volumeBindingMode set to WaitForFirstConsumer, won't work as expected_

### Creating a scheduled backup
To create a scheduled backup, run the following command

```
velero create schedule newschedule  --schedule="*/5 * * * *" --snapshot-volumes --include-namespaces=default --volume-snapshot-locations=<SNAPSHOT_LOCATION>
```

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-local-volumesnapshotlocation.yaml`.

You can check the status of scheduled using the following command:

```
velero schedule get
```

It will list all the schedule you created. Sample output of the above command is as below:
```
NAME            STATUS    CREATED                         SCHEDULE      BACKUP TTL   LAST BACKUP   SELECTOR
newschedule     Enabled   2019-05-13 15:15:39 +0530 IST   */5 * * * *   720h0m0s     2m ago        <none>
```

#### Creating a restore from scheduled backup
To restore from any scheduled backup, refer [Creating a restore](#creating-a-restore)

## Remote Backup/Restore
For Remote Backup Velero-plugin creates a snapshot for CStor Volume and upload it to remote storage.

### Configuring snapshot location for remote backup
To take remote backup of cStor volume snapshot to cloud or S3 compatible storage, configure VolumeSnapshotLocation with provider `openebs.io/cstor-blockstore`. Sample YAML file for volumesnapshotlocation can be found at `example/06-volumesnapshotlocation.yaml`.

Sample Spec for volumesnapshotlocation:

```yaml
spec:
  provider: openebs.io/cstor-blockstore
  config:
    bucket: <YOUR_BUCKET>
    prefix: <PREFIX_FOR_BACKUP_NAME>
    backupPathPrefix: <PREFIX_FOR_BACKUP_PATH>
    provider: <GCP_OR_AWS>
    region: <AWS_REGION>
```

If you have multiple installation of openebs then you need to add `spec.config.namespace: <OPENEBS_NAMESPACE>`.

*Note:*

- _`prefix` is for the backup file name._

  _if `prefix` is set to `cstor` then snapshot will be stored as `bucket/backups/backup_name/cstor-PV_NAME-backup_name`._
- _`backupPathPrefix` is for backup path._

  _if `backupPathPrefix` is set to `newcluster` then snapshot will be stored at `bucket/newcluster/backups/backup_name/prefix-PV_NAME-backup_name`._

  _To store backup metadata and snapshot at same location, `BackupStorageLocation.prefix` and `VolumeSnapshotLocation.BackupPathPrefix` should be same._

You can configure a backup storage location(`BackupStorageLocation`) similarly.
Currently supported cloud-providers for velero-plugin are AWS, GCP and MinIO.

### Creating a remote backup
To back up data of all your applications in the default namespace, run the following command:

```
velero backup create defaultbackup --include-namespaces=default --snapshot-volumes --volume-snapshot-locations=<SNAPSHOT_LOCATION>
```

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-volumesnapshotlocation.yaml`.

You can check the status of backup using the following command:

```
velero backup get
```

Above command will list out the all backups you created. Sample output of the above command is mentioned below :
```
NAME                STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
defaultbackup       Completed   2019-05-09 17:08:41 +0530 IST   26d       gcp                <none>
```
Once the backup is completed you should see the backup marked as `Completed`.

*Note:*
- _If backup name ends with "-20190513104034" format then it is considered as part of scheduled backup_

#### Creating a restore for remote backup
To restore data from remote backup, run the following command:

```
velero restore create --from-backup backup_name --restore-volumes=true
```

With the above command, the plugin will create a CStor volume and the data from backup will be restored on this newly created volume.

You can check the status of restore using the following command:

```
velero restore get
```

Above command will list out the all restores you created. Sample output of the above command is mentioned below :
```
NAME                           BACKUP          STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
defaultbackup-20190513113453   defaultbackup   Completed   0          0         2019-05-13 11:34:55 +0530 IST   <none>
```

Once the restore is completed you should see the restore marked as `Completed`.


To restore in different namespace, run the following command:

```
velero restore create --from-backup backup_name --restore-volumes=true --namespace-mappings source_ns:destination_ns
```

Plugin will create the destination_ns, if it doesn't exist.

**Once restore for remote backup is completed, You need to set targetip in relevant replica. Refer [Setting targetip in replica](#setting-targetip-in-replica).**

#### Setting targetip in replica
After restore for remote backup is completed, you need to set target-ip for the volume in pool pod. If restore is from local snapshot then you don't need to update target-ip
- Fetch the targetip for replica using below command.
```
kubectl get svc -n openebs <PV_NAME> -ojsonpath='{.spec.clusterIP}'
```
PV_NAME is restored PV name.

- After getting targetip, you need to set it in all the replica of restored pv using following command:
```
kubectl exec -it <POOL_POD> -c cstor-pool -n openebs -- bash
zfs set io.openebs:targetip=<TARGET_IP> <POOL_NAME/VOLUME_NAME>
```

- Using bash script
```shell script
#set correct pod name
pool_pod=POOL_POD

for pool_pvc in $(kubectl exec $pool_pod -c cstor-pool -n openebs -- bash -c "zfs get io.openebs:targetip" | grep io.openebs:targetip | grep pvc | grep -v '@' | cut -d" " -f1)
do
  svc=$(echo $pool_pvc | cut -d/ -f2)
  ip=$(kubectl get svc -n openebs $svc -ojsonpath='{.spec.clusterIP}')
  kubectl exec $pool_pod -c cstor-pool -n openebs -- bash -c "zfs set io.openebs:targetip=$ip $pool_pvc"
done
```

You can automate this process by setting the config parameter `autoSetTargetIP` to `"true"` in volumesnapshotlocation.
Note that `restoreAllIncrementalSnapshots=true` implies `autoSetTargetIP=true`

```
apiVersion: velero.io/v1
kind: VolumeSnapshotLocation
metadata:
  ...
spec:
  config:
    ...
    ...
    autoSetTargetIP: "true"
```

### Creating a scheduled remote backup
OpenEBS velero-plugin provides incremental remote backup support for CStor persistent volumes for scheduled backups. This means, the first backup of the schedule includes a snapshot of all volume data, and the subsequent backups include the snapshot of modified data from the previous backup

To create an incremental backup(or scheduled backup), run the following command:

```
velero create schedule newschedule  --schedule="*/5 * * * *" --snapshot-volumes --include-namespaces=default --volume-snapshot-locations=<SNAPSHOT_LOCATION>
```

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-volumesnapshotlocation.yaml`.

You can check the status of scheduled using the following command:

```
velero schedule get
```

It will list all the schedule you created. Sample output of the above command is as below:
```
NAME            STATUS    CREATED                         SCHEDULE      BACKUP TTL   LAST BACKUP   SELECTOR
newschedule     Enabled   2019-05-13 15:15:39 +0530 IST   */5 * * * *   720h0m0s     2m ago        <none>
```

During the first backup iteration of a schedule, full data of the volume will be backed up. For later backup iterations of a schedule, only modified or new data from the previous iteration will be backed up. Since Velero backup comes with [retain policy](https://velero.io/docs/main/how-velero-works/#set-a-backup-to-expire), you may need to update the retain policy using argument `--ttl` while creating a schedule. Since scheduled backups are incremental backup, if first backup(or base backup) gets expired then you won't be able to restore from that schedule. 

*Note:*
- _If backup name ends with "-20190513104034" format then it is considered as part of scheduled backup_

#### Creating a restore from scheduled remote backup
Backups generated by schedule are incremental backups. The first backup of the schedule includes a snapshot of all volume data, and the subsequent backups include the snapshot of modified data from the previous backup. In the older version of velero-plugin(<2.2.0) we need to create restore for all the backup, from base backup to the required backup, Refer [Restoring the scheduled backup without restoreAllIncrementalSnapshots](#restoring-the-scheduled-backup-without-restoreallincrementalsnapshots).

You can automate this process by setting the config parameter `restoreAllIncrementalSnapshots` to `"true"` in volumesnapshotlocation.

```
apiVersion: velero.io/v1
kind: VolumeSnapshotLocation
metadata:
  ...
spec:
  config:
    ...
    ...
    restoreAllIncrementalSnapshots: "true"
```
To create restore from schedule, run the following command

```
velero restore create --from-schedule schedule_name --restore-volumes=true
```

The above command will create the cstor volume and restore all the snapshots backed up in that schedule.


To restore specific backup from schedule, run the following command

```
velero restore create --from-backup backup_name --restore-volumes=true
```

Above command will create the cstor volume and restore all the snapshots backed up from base backup to given backup(backup_name).

Here, base backup means the first backup created by schedule. To restore from scheduled backups, base-backup must be available.

You can restore the scheduled remote backup to a different namespace using the `--namespace-mappings` argument while creating a restore. Plugin will create the destination namespace, if it doesn't exist.

Once restore for remote scheduled backup is completed, You need to set targetip in relevant replica. Refer [Setting targetip in replica](#setting-targetip-in-replica).

If you are not setting `restoreAllIncrementalSnapshots` parameter in volumesnapshotlocation then follow the [below section](#restoring-the-scheduled-backup-without-restoreallincrementalsnapshots
) to restore from scheduled backups.

#### Restoring the scheduled backup without restoreAllIncrementalSnapshots

Since backups taken are incremental for a schedule, the order of restoring data is very important. You need to restore data in the order of the backups created. 

First restore must be created from the first completed backup of schedule.

For example, below are the available backups for a schedule:
```
NAME                   STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
sched-20190513104034   Completed   2019-05-13 16:10:34 +0530 IST   29d       gcp                <none>
sched-20190513103534   Completed   2019-05-13 16:05:34 +0530 IST   29d       gcp                <none>
sched-20190513103034   Completed   2019-05-13 16:00:34 +0530 IST   29d       gcp                <none>
```

Restore of data needs to be done in the following way:
```
velero restore create --from-backup sched-20190513103034 --restore-volumes=true
velero restore create --from-backup sched-20190513103534 --restore-volumes=true
velero restore create --from-backup sched-20190513104034 --restore-volumes=true
```

You can restore scheduled remote backup to different namespace using `--namespace-mappings` argument [while creating a restore](#creating-a-restore-for-remote-backup).

Once restore for remote scheduled backup is completed, You need to set targetip in relevant replica. Refer [Setting targetip in replica](#setting-targetip-in-replica).

*Note: Velero clean-up the backups according to retain policy. By default retain policy is 30days. So you need to set retain policy for scheduled remote/cloud-backup accordingly.*

## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_large)
