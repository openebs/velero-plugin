# Velero-plugin for OpenEBS CStor volume

Velero is a utility to back up and restore your Kubernetes resource and persistent volumes.

To do backup/restore of OpenEBS CStor volumes through Velero utility, you need to install and configure
OpenEBS velero-plugin.

[![Build Status](https://travis-ci.org/openebs/velero-plugin.svg?branch=master)](https://travis-ci.org/openebs/velero-plugin)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_shield)

## Table of Contents
- [Compatibility matrix](#compatibility-matrix)
- [Prerequisite for velero-plugin](#prerequisite-for-velero-plugin)
- [Installation of velero-plugin](#installation-of-velero-plugin)
- [Configuring snapshot location](#configuring-snapshot-location)
  - [Configuring snapshot location for remote backup](#configuring-snapshot-location-for-remote-backup)
  - [Configuring snapshot location for local backup](#configuring-snapshot-location-for-local-backup)
- [Managing Backups](#managing-backups)
  - [Creating a backup](#creating-a-backup)
  - [Creating a restore from backup](#creating-a-restore-from-backup)
    - [Creating a restore from local backup](#creating-a-restore-from-local-backup)
    - [Creating a restore from remote backup](#creating-a-restore-from-remote-backup)
  - [Creating a scheduled backup](#creating-a-scheduled-backup-or-incremental-backup-for-cstor-volume)
  - [Restoring from a scheduled backup](#restoring-from-a-scheduled-backup)
    - [Restoring from a scheduled local backup](#restoring-from-a-scheduled-local-backup)
    - [Restoring from a scheduled remote backup](#restoring-from-a-scheduled-remote-backup)

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

Plugin images are available at [quay.io](http://quay.io/openebs/velero-plugin) and [hub.docker.com](https://hub.docker.com/r/openebs/velero-plugin/tags).

## Prerequisite for velero-plugin
A Specific version of Velero needs to be installed as per the [compatibility matrix](#Compatibility-matrix) with OpenEBS versions.

For installation steps of Velero, visit https://velero.io.

For installation steps of OpenEBS, visit https://github.com/openebs/openebs/releases.

## Installation of velero-plugin
Run the following command to install development image of OpenEBS velero-plugin

`velero plugin add openebs/velero-plugin:1.9.0`

This command will add an init container to Velero deployment to install the OpenEBS velero-plugin.

## Configuring snapshot location
Velero-plugin support two types of backup:
1. Remote Backup

	In case of remote backup Velero-plugin creates a snapshot on CStor volume and upload it to remote storage.

2. Local Backup

	In case of local backup Velero-plugin creates a snapshot on CStor volume.

### Configuring snapshot location for remote backup
To take remote backup of cStor volume snapshot to cloud or S3 compatible storage, configure VolumeSnapshotLocation with provider `openebs.io/cstor-blockstore`. Sample YAML file for volumesnapshotlocation can be found at `example/06-volumesnapshotlocation.yaml`.

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

### Configuring snapshot location for local backup
For local backup(/snapshot) of CStor volume through velero, Refer `example/06-local-volumesnapshotlocation.yaml` to configure `VolumeSnapshotLocation`.

For local backup Velero version must be >= v1.3.0, velero-plugin >=1.9.0 and OpenEBS >= 1.9.0.

```yaml
spec:
  provider: openebs.io/cstor-blockstore
  config:
    namespace: <OPENEBS_NAMESPACE>
    local: "true"
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

## Managing Backups
Once the volumesnapshotlocation is configured, you can create the backup/restore of your CStor persistent storage volume.

### Creating a backup
To back up data of all your applications in the default namespace, run the following command:

`velero backup create defaultbackup --include-namespaces=default --snapshot-volumes --volume-snapshot-locations=<SNAPSHOT_LOCATION>`

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-volumesnapshotlocation.yaml` or `example/06-local-volumesnapshotlocation.yaml`.

You can check the status of backup using the following command:

`velero backup get `

Above command will list out the all backups you created. Sample output of the above command is mentioned below :
```
NAME                STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
defaultbackup       Completed   2019-05-09 17:08:41 +0530 IST   26d       gcp                <none>
```
Once the backup is completed you should see the backup marked as `Completed`.

*Note:*
- _If backup name ends with "-20190513104034" format then it is considered as part of scheduled backup_


### Creating a restore from backup
#### Creating a restore from local backup
To restore local backup, run the following command:

`velero restore create --from-backup backup_name --restore-volumes=true --namespace-mappings source_ns:destination_ns`

*Note:*
- _Restore from local backup need to be done from same cluster only where local backups are created, unlike restore of remote backups which can be done to any cluster_

*Limitation:*
- _Restore of PV having storageClass, with volumeBindingMode set to WaitForFirstConsumer, won't work as expected_

#### Creating a restore from remote backup
To restore data from remote/cloud backup, run the following command:

`velero restore create --from-backup backup_name --restore-volumes=true`

With the above command, the plugin will create a CStor volume and the data from backup will be restored on this newly created volume.

Note: You need to mention `--restore-volumes=true` while doing a restore.

You can check the status of restore using the following command:

`velero restore get`

Above command will list out the all restores you created. Sample output of the above command is mentioned below :
```
NAME                           BACKUP          STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
defaultbackup-20190513113453   defaultbackup   Completed   0          0         2019-05-13 11:34:55 +0530 IST   <none>
```
Once the restore is completed you should see the restore marked as `Completed`.

*Note: After restore for remote/cloud-backup is completed, you need to set target-ip for the volume in pool pod. If restore is from local snapshot then you don't need to update target-ip*
*Steps to get target-ip*
1. kubectl get svc -n openebs <PV_NAME> -ojsonpath='{.spec.clusterIP}'
*Steps to update `target-ip` in pool pod is as follow:*
```
1. kubectl exec -it <POOL_POD> -c cstor-pool -n openebs -- bash
2. zfs set io.openebs:targetip=<TARGET_IP> <POOL_NAME/VOLUME_NAME>
```
*Limitation:*
- _Restore of remote/cloud-backup in different namespace(--namespace-remapping) is not supported_

### Creating a scheduled backup (or incremental backup for CStor volume)
OpenEBS velero-plugin provides incremental backup support for CStor persistent volumes.
To create an incremental backup(or scheduled backup), run the following command:

`velero create schedule newschedule  --schedule="*/5 * * * *" --snapshot-volumes --include-namespaces=default --volume-snapshot-locations=<SNAPSHOT_LOCATION>`

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-volumesnapshotlocation.yaml`.

You can check the status of scheduled using the following command:

`velero schedule get`

It will list all the schedule you created. Sample output of the above command is as below:
```
NAME            STATUS    CREATED                         SCHEDULE      BACKUP TTL   LAST BACKUP   SELECTOR
newschedule     Enabled   2019-05-13 15:15:39 +0530 IST   */5 * * * *   720h0m0s     2m ago        <none>
```

During the first backup iteration of a schedule, full data of the volume will be backed up. For later backup iterations of a schedule, only modified or new data from the previous iteration will be backed up.

*Note:*
- _If backup name ends with "-20190513104034" format then it is considered as part of scheduled backup_

### Restoring from a scheduled backup
#### Restoring from a scheduled local backup
In case of local backups/snapshots, you can restore from any completed backup created by the schedule.

#### Restoring from a scheduled remote backup
Since backups taken are incremental for a schedule, the order of restoring data is important. You need to restore data in the order of the backups created. First restore must be created from the first completed backup of schedule.

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
*Note: Velero clean-up the backups according to retain policy. By default retain policy is 30days. So you need to set retain policy for scheduled remote/cloud-backup accordingly.*

## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopenebs%2Fvelero-plugin?ref=badge_large)
