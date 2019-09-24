# Executing the integration test cases
## Table of Contents
- [Prerequisite for integration tests](#prerequisite-for-integration-tests)
- [Configuring integration tests](#configuring-integration-tests)
  - [Application](#application)
  - [OpenEBS](#openebs)
    - [StorageClass](#storageclass)
    - [StoragePoolClaim](#storagepoolclaim)
    - [PersistentVolumeClaim](#persistentvolumeclaim)
  - [Velero](#velero)
    - [BackupStorageLocation](#backupstoragelocation)
    - [VolumeSnapshotLocation](#volumesnapshotlocation)
- [Executing integration test](#executing-integration-test)

## Prerequisite for integration tests
To execute the integration test cases under `velero-plugin/tests`, you need to have a working installation of the following components.
1. OpenEBS
2. Velero
3. openebs/velero-plugin.

## Configuring integration tests
### Application
By default, `velero-plugin/tests/sanity` creates a namespace `test` to deploy the application.
you can configure the application by updating the variable `velero-plugin/tests/app.BusyboxYaml`.
To update the volume configuration, check PersistentVolumeClaim section.

### OpenEBS
`velero-plugin/tests/sanity` assumes that OpenEBS is installed in a namespace `openebs`. If you have installed OpenEBS in different a namespace then you need to update the variable `velero-plugin/tests/openebs/OpenEBSNs` accordingly.

#### StorageClass
`velero-plugin/tests/sanity` creates storageClass `openebs-cstor-sparse-auto` having replicaCount as 1 for cStor Volume. You can configure the storageClassing by updating the variable `velero-plugin/tests/openebs/SCYaml`.

#### StoragePoolClaim
`velero-plugin/tests/sanity` creates StoragePoolClaim `sparse-claim-auto` for cStor Volume. You can configure the StoragePoolClaim by updating the variable `velero-plugin/tests/openebs/SPCYaml`.

#### PersistentVolumeClaim
`velero-plugin/tests/sanity` creates PersistentVolumeClaim `cstor-vol1-1r-claim` for cStor Volume. You can configure the PersistentVolumeClaim by updating the variable `velero-plugin/tests/openebs/PVCYaml`.

### Velero
`velero-plugin/tests/sanity` assumes that Velero is installed in a namespace `velero`. If you have installed Velero in different a namespace then you need to update the variable `velero-plugin/tests/velero/VeleroNamespace` accordingly.

#### BackupStorageLocation
The default value of `BackupStorageLocation` in tests is `default`. If you have different `BackupStorageLocation` then you need to update the variable `velero-plugin/tests/sanity/BackupLocation`.

#### VolumeSnapshotLocation
The default value of `VolumeSnapshotLocation` in tests is `default`. If you have different `VolumeSnapshotLocation` then you need to update the variable `velero-plugin/tests/sanity/SnapshotLocation`.


## Executing integration test
To execute the test under `velero-plugin/tests`, execute the following command:

`make test`

or

`go test -v  ./tests/sanity/...`
