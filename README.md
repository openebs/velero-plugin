## Overview
[![Build Status](https://travis-ci.org/openebs/velero-plugin.svg?branch=master)](https://travis-ci.org/openebs/velero-plugin)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)

Velero is a utility to back up and restore your Kubernetes resource and persistent volumes.

To do backup/restore of OpenEBS CStor volumes through Velero utility, you need to install and configure
OpenEBS velero-plugin.

## Prerequisite for velero-plugin
Specific version of Velero needs to be installed as per the [compatibility matrix](#Compatibility-matrix) with OpenEBS versions.

For installation steps of Velero, visit https://heptio.github.io/velero.

For installation steps of OpenEBS, visit https://github.com/openebs/openebs/releases.

## Installation of velero-plugin
Run the following command to install OpenEBS velero-plugin

`velero plugin add openebs/velero-plugin:ci`

This command will add an init container to Velero deployment to install the OpenEBS velero-plugin.

## Taking backup of CStor volume data through the Velero
To take a backup of CStor volume through Velero, configure `VolumeSnapshotLocation` with provider `cstor-blockstore`. Sample yaml file for volumesnapshotlocation can be found at `example/06-volumesnapshotlocation.yaml`.

```
spec:
  provider: cstor-blockstore
  config:
    bucket: <YOUR_BUCKET>
    prefix: <PREFIX_FOR_BACKUP_NAME>
    provider: <GCP_OR_AWS>
    region: <AWS_REGION>
```

You can configure a backup storage location(`BackupStorageLocation`) in similar way. Currently, AWS and GCP are supported.


## Managing Backups
Once the volumesnapshot location is configured, you can create the backup/restore of your CStor persistent storage volume.

### Creating a backup
To back up data of all your applications in the default namespace, run the following command:

`velero backup create defaultbackup --include-namespaces=default --snapshot-volumes --volume-snapshot-locations=<SNAPSHOT_LOCATION>`

`SNAPSHOT_LOCATION` should be the same as you configured by using `example/06-volumesnapshotlocation.yaml`.

You can check the status of backup using the following command:

`velero backup get `

Above command will list out the all backups you created. Sample output of the above command is mentioned below :
```
NAME                STATUS      CREATED                         EXPIRES   STORAGE LOCATION   SELECTOR
defaultbackup       Completed   2019-05-09 17:08:41 +0530 IST   26d       gcp                <none>
```
Once the backup is completed you should see the backup marked as `Completed`.


### Creating a restore from backup
To restore data from backup, run the following command:

`velero restore create --from-backup backup_name --restore-volumes=true`

With above command, plugin will create a CStor volume and the data from backup will be restored on this newly created volume.

Note: You need to mention `--restore-volumes=true` while doing a restore.

You can check the status of restore using the following command:

`velero restore get`

Above command will list out the all restores you created. Sample output of the above command is mentioned below :
```
NAME                           BACKUP          STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
defaultbackup-20190513113453   defaultbackup   Completed   0          0         2019-05-13 11:34:55 +0530 IST   <none>
```
Once the restore is completed you should see the restore marked as `Completed`.


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

### To restore from a schedule
Since backups taken are incremental for a schedule, order of restoring data is important. You need to restore data in the order of the backups created.

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

## Compatibility matrix

|     Image           |    Codebase     |  Velero v0.10.0  | Velero v0.11.0 |
|   ---------------   |  -------------  |   ----------------      |  --------------- |
| ark-plugin:0.9.0-RC2    |     v0.9.x       |         ✓            |                    |
| velero-plugin:ci       |     master    |                     |         ✓           |

Plugin images are available at quay.io and hub.docker.com.