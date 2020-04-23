# Velero-plugin for OpenEBS CStor volume

Velero is a utility to back up and restore your Kubernetes resource and persistent volumes.

To do backup/restore of OpenEBS CStor volumes through Velero utility, you need to install and configure
OpenEBS velero-plugin.

[![Build Status](https://travis-ci.org/openebs/velero-plugin.svg?branch=master)](https://travis-ci.org/openebs/velero-plugin)
[![Slack](https://img.shields.io/badge/chat!!!-slack-ff1493.svg?style=flat-square)](https://kubernetes.slack.com/messages/openebs)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_shield)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3900/badge)](https://bestpractices.coreinfrastructure.org/projects/3900)

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

*Note:*

_OpenEBS version **< 0.9** is not supported for velero-plugin._

_If you want to use plugin image from development branch(`master`), use **ci** tag._

Plugin images are available at:

For AMD64: [quay.io](http://quay.io/openebs/velero-plugin) and [hub.docker.com](https://hub.docker.com/r/openebs/velero-plugin/tags).

For ARM64: [quay.io](https://quay.io/repository/openebs/velero-plugin-arm64?tab=tags) and [hub.docker.com](https://hub.docker.com/r/openebs/velero-plugin-arm64/tags).

## Prerequisite for velero-plugin
A Specific version of Velero needs to be installed as per the [compatibility matrix](#Compatibility-matrix) with OpenEBS versions.

For installation steps of Velero, visit https://velero.io.

For installation steps of OpenEBS, visit https://github.com/openebs/openebs/releases.

## Installation of velero-plugin
Run the following command to install development image of OpenEBS velero-plugin

`velero plugin add openebs/velero-plugin:1.9.0`

For ARM64, change image name to `openebs/velero-plugin-arm64`.

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
Before restoring from remote backup, make sure that you have created the namespace in your destination cluster.

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

*Note: After restore for remote backup is completed, you need to set target-ip for the volume in pool pod. If restore is from local snapshot then you don't need to update target-ip*
*Steps to get target-ip*
1. kubectl get svc -n openebs <PV_NAME> -ojsonpath='{.spec.clusterIP}'
*Steps to update `target-ip` in pool pod is as follow:*
```
1. kubectl exec -it <POOL_POD> -c cstor-pool -n openebs -- bash
2. zfs set io.openebs:targetip=<TARGET_IP> <POOL_NAME/VOLUME_NAME>
```

### Creating a scheduled remote backup
OpenEBS velero-plugin provides incremental remote backup support for CStor persistent volumes.

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

During the first backup iteration of a schedule, full data of the volume will be backed up. For later backup iterations of a schedule, only modified or new data from the previous iteration will be backed up. Since Velero backup comes with [retain policy](https://velero.io/docs/master/how-velero-works/#set-a-backup-to-expire), you may need to update the retain policy using argument `--ttl` while creating a schedule.

*Note:*
- _If backup name ends with "-20190513104034" format then it is considered as part of scheduled backup_

#### Creating a restore from scheduled remote backup
Before restoring from remote backup, make sure that you have created the namespace in your destination cluster.

Since backups taken are incremental for a schedule, the order of restoring data is very important. You need to restore data in the order of the backups created. 

First restore must be created from the first completed backup of schedule.

For example, below are the available backups for a schedule:
```
NAME                   STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
sched-20190513104034   Completed   2019-05-13 16:10:34 +0530 IST   29d       gcp                <none>
sched-20190513103534   Completed   2019-05-13 16:05:34 +0530 IST   29d       gcp                <none>
sched-20190513103034   Completed   2019-05-13 16:00:34 +0530 IST   29d       gcp                <none>
```

Restore of data need to be done in following way:
```
velero restore create --from-backup sched-20190513103034 --restore-volumes=true
velero restore create --from-backup sched-20190513103534 --restore-volumes=true
velero restore create --from-backup sched-20190513104034 --restore-volumes=true
```

You can restore scheduled remote backup to different namespace using `--namespace-mappings` argument [while creating a restore](#creating-a-restore-for-remote-backup).

*Note: Velero clean-up the backups according to retain policy. By default retain policy is 30days. So you need to set retain policy for scheduled remote/cloud-backup accordingly.*

## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_large)
