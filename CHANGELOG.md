v2.11.0 / 2021-07-14
========================


v2.11.0-RC2 / 2021-07-12
========================


v2.11.0-RC1 / 2021-07-11
========================


v2.10.0 / 2021-06-14
========================


v2.10.0-RC2 / 2021-06-11
========================


v2.10.0-RC1 / 2021-06-09
========================


v2.9.0 / 2021-05-13
========================
* Adding a new VolumeSnapshotLocation config parameter restApiTimeout to configure timeout for HTTP REST request between plugin and cstor services. ([#154](https://github.com/openebs/velero-plugin/pull/154),[@mynktl](https://github.com/mynktl))
* refact(deps): bump zfslocalpv, maya, api and k8s client-go dependencies ([#161](https://github.com/openebs/velero-plugin/pull/161),[@prateekpandey14](https://github.com/prateekpandey14))


v2.9.0-RC2 / 2021-05-10
========================


v2.9.0-RC1 / 2021-05-07
========================
* Adding a new VolumeSnapshotLocation config parameter restApiTimeout to configure timeout for HTTP REST request between plugin and cstor services. ([#154](https://github.com/openebs/velero-plugin/pull/154),[@mynktl](https://github.com/mynktl))
* refact(deps): bump zfslocalpv, maya, api and k8s client-go dependencies ([#161](https://github.com/openebs/velero-plugin/pull/161),[@prateekpandey14](https://github.com/prateekpandey14))


v2.8.0 / 2021-04-14
========================


v2.8.0-RC2 / 2021-04-12
========================


v2.8.0-RC1 / 2021-04-07
========================


v2.7.0 / 2021-03-11
========================
* moved travis tests to github action and removed travis.yml ([#149](https://github.com/openebs/velero-plugin/pull/149),[@shubham14bajpai](https://github.com/shubham14bajpai))
* adding support to restore in an encrypted pool for ZFS-LocalPV ([#147](https://github.com/openebs/velero-plugin/pull/147),[@pawanpraka1](https://github.com/pawanpraka1))


v2.7.0-RC2 / 2021-03-10
========================


v2.7.0-RC1 / 2021-03-08
========================
* moved travis tests to github action and removed travis.yml ([#149](https://github.com/openebs/velero-plugin/pull/149),[@shubham14bajpai](https://github.com/shubham14bajpai))
* adding support to restore in an encrypted pool for ZFS-LocalPV ([#147](https://github.com/openebs/velero-plugin/pull/147),[@pawanpraka1](https://github.com/pawanpraka1))


v2.6.0 / 2021-02-13
========================


v2.6.0-RC2 / 2021-02-11
========================


v2.6.0-RC1 / 2021-02-08
========================


v2.5.0 / 2021-01-13
========================
* Removing wait on setting target-ip for CVR. This fixes the restore for application pod having target-affinity set. ([#144](https://github.com/openebs/velero-plugin/pull/140),[@mynktl](https://github.com/mynktl))


v2.5.0-RC2 / 2021-01-11
========================


v2.5.0-RC1 / 2021-01-08
========================
* Removing wait on setting target-ip for CVR. This fixes the restore for application pod having target-affinity set. ([#144](https://github.com/openebs/velero-plugin/pull/140),[@mynktl](https://github.com/mynktl))


v2.4.0 / 2020-12-13
========================
* Marking CVRs after successful restore with openebs.io/restore-completed if autoSetTargetIP=true or restoreAllIncrementalSnapshots=true ([#131](https://github.com/openebs/velero-plugin/pull/131),[@zlymeda](https://github.com/zlymeda))
* updating the label selector for restoring the ZFS-LocalPV volumes on different node ([#139](https://github.com/openebs/velero-plugin/pull/139),[@pawanpraka1](https://github.com/pawanpraka1))
* Adding support to create destination namespace, for restore, if it doesn't exist ([#140](https://github.com/openebs/velero-plugin/pull/140),[@mynktl](https://github.com/mynktl))


v2.4.0-RC2 / 2020-12-12
========================


v2.4.0-RC1 / 2020-12-10
========================
* Marking CVRs after successful restore with openebs.io/restore-completed if autoSetTargetIP=true or restoreAllIncrementalSnapshots=true ([#131](https://github.com/openebs/velero-plugin/pull/131),[@zlymeda](https://github.com/zlymeda))
* updating the label selector for restoring the ZFS-LocalPV volumes on different node ([#139](https://github.com/openebs/velero-plugin/pull/139),[@pawanpraka1](https://github.com/pawanpraka1))
* Adding support to create destination namespace, for restore, if it doesn't exist ([#140](https://github.com/openebs/velero-plugin/pull/140),[@mynktl](https://github.com/mynktl))


v2.3.0 / 2020-11-14
========================
* Multi-arch container image support for velero plugin. Migrate the multi-arch builds to github-action to support amd64 and arm64 architectures. ([#133](https://github.com/openebs/velero-plugin/pull/133),[@prateek](https://github.com/prateek))
* Adding github action workflows to build multiarch images using docker buildx. ([#132](https://github.com/openebs/velero-plugin/pull/132),[@shubham14bajpai](https://github.com/shubham14bajpai))
* Adding new config parameter "restoreAllIncrementalSnapshots" to restore all the scheduled backups, from base backup to the given backup, using single restore ([#99](https://github.com/openebs/velero-plugin/pull/99),[@mynktl](https://github.com/mynktl))


v2.3.0-RC2 / 2020-11-13
========================


v2.3.0-RC1 / 2020-11-13
========================
* Multi-arch container image support for velero plugin. Migrate the multi-arch builds to github-action to support amd64 and arm64 architectures. ([#133](https://github.com/openebs/velero-plugin/pull/133),[@prateek](https://github.com/prateek))
* Adding github action workflows to build multiarch images using docker buildx. ([#132](https://github.com/openebs/velero-plugin/pull/132),[@shubham14bajpai](https://github.com/shubham14bajpai))
* Adding new config parameter "restoreAllIncrementalSnapshots" to restore all the scheduled backups, from base backup to the given backup, using single restore ([#99](https://github.com/openebs/velero-plugin/pull/99),[@mynktl](https://github.com/mynktl))


v2.2.0 / 2020-10-13
========================
* use schedule name to identify the scheduled backup for ZFS-LocalPV ([#124](https://github.com/openebs/velero-plugin/pull/124),[@pawanpraka1](https://github.com/pawanpraka1))
* fixing the backup deletion for ZFS-LocalPV ([#128](https://github.com/openebs/velero-plugin/pull/128),[@pawanpraka1](https://github.com/pawanpraka1))
* Added support to use custom certificate and option to skip certificate verification for s3 object storage ([#115](https://github.com/openebs/velero-plugin/pull/115),[@mynktl](https://github.com/mynktl))
* adding support to restore on different setup/nodes ([#118](https://github.com/openebs/velero-plugin/pull/118),[@pawanpraka1](https://github.com/pawanpraka1))
* making log level available for velero plugin ([#116](https://github.com/openebs/velero-plugin/pull/116),[@pawanpraka1](https://github.com/pawanpraka1))
* wait for plugin server to be ready before doing backup/restore ([#117](https://github.com/openebs/velero-plugin/pull/117),[@pawanpraka1](https://github.com/pawanpraka1))
* adding support to do incremental backup/restore for ZFS-LocalPV ([#121](https://github.com/openebs/velero-plugin/pull/121),[@pawanpraka1](https://github.com/pawanpraka1))


v2.2.0-RC2 / 2020-10-12
========================
* use schedule name to identify the scheduled backup for ZFS-LocalPV ([#124](https://github.com/openebs/velero-plugin/pull/124),[@pawanpraka1](https://github.com/pawanpraka1))
* fixing the backup deletion for ZFS-LocalPV ([#128](https://github.com/openebs/velero-plugin/pull/128),[@pawanpraka1](https://github.com/pawanpraka1))


v2.2.0-RC1 / 2020-10-08
========================
* Added support to use custom certificate and option to skip certificate verification for s3 object storage ([#115](https://github.com/openebs/velero-plugin/pull/115),[@mynktl](https://github.com/mynktl))
* adding support to restore on different setup/nodes ([#118](https://github.com/openebs/velero-plugin/pull/118),[@pawanpraka1](https://github.com/pawanpraka1))
* making log level available for velero plugin ([#116](https://github.com/openebs/velero-plugin/pull/116),[@pawanpraka1](https://github.com/pawanpraka1))
* wait for plugin server to be ready before doing backup/restore ([#117](https://github.com/openebs/velero-plugin/pull/117),[@pawanpraka1](https://github.com/pawanpraka1))
* adding support to do incremental backup/restore for ZFS-LocalPV ([#121](https://github.com/openebs/velero-plugin/pull/121),[@pawanpraka1](https://github.com/pawanpraka1))


v2.1.0 / 2020-09-15
========================
* adding support for parallel backup and restore ([#111](https://github.com/openebs/velero-plugin/pull/111),[@pawanpraka1](https://github.com/pawanpraka1))
* adding velero plugin for ZFS-LocalPV ([#102](https://github.com/openebs/velero-plugin/pull/102),[@pawanpraka1](https://github.com/pawanpraka1))
* making directory and binary name openebs specific ([#110](https://github.com/openebs/velero-plugin/pull/110),[@pawanpraka1](https://github.com/pawanpraka1))
* Add support for local restore of cStor CSI volume ([#108](https://github.com/openebs/velero-plugin/pull/108),[@mittachaitu](https://github.com/mittachaitu))


v2.0.0 / 2020-08-14
========================


v1.12.0 / 2020-07-14
========================


v1.11.0 / 2020-06-12
========================
* add restore support for cStor CSI based volumes ([#93](https://github.com/openebs/velero-plugin/pull/93),[@sonasingh46](https://github.com/sonasingh46))


v1.11.0-RC2 / 2020-06-11
========================
* add restore support for cStor CSI based volumes ([#93](https://github.com/openebs/velero-plugin/pull/93),[@sonasingh46](https://github.com/sonasingh46))


v1.11.0-RC1 / 2020-06-10
========================


v1.10.0 / 2020-05-15
========================
* Fixing failure in restoring restic backup of cstor volumes, created by velero-plugin restore [issue:84](https://github.com/openebs/velero-plugin/issues/84) ([#85](https://github.com/openebs/velero-plugin/pull/85),[@mynktl](https://github.com/mynktl))
* Adding support to restore remote backup in different namespace ([#72](https://github.com/openebs/velero-plugin/pull/72),[@mynktl](https://github.com/mynktl))
* Adding support for multiple s3 profile to backup cstor volumes to different s3 location ([#76](https://github.com/openebs/velero-plugin/pull/76),[@mynktl](https://github.com/mynktl))
* Fixed panic, created because of empty snapshotID, while deleting failed backup ([#79](https://github.com/openebs/velero-plugin/pull/79),[@mynktl](https://github.com/mynktl))


v1.10.0-RC2 / 2020-05-13
========================
* Fixing failure in restoring restic backup of cstor volumes, created by velero-plugin restore [issue:84](https://github.com/openebs/velero-plugin/issues/84) ([#85](https://github.com/openebs/velero-plugin/pull/85),[@mynktl](https://github.com/mynktl))


v1.10.0-RC1 / 2020-05-08
========================
* Adding support to restore remote backup in different namespace ([#72](https://github.com/openebs/velero-plugin/pull/72),[@mynktl](https://github.com/mynktl))
* Adding support for multiple s3 profile to backup cstor volumes to different s3 location ([#76](https://github.com/openebs/velero-plugin/pull/76),[@mynktl](https://github.com/mynktl))
* Fixed panic, created because of empty snapshotID, while deleting failed backup ([#79](https://github.com/openebs/velero-plugin/pull/79),[@mynktl](https://github.com/mynktl))



1.9.0 / 2020-04-14
========================
* ARM build for velero-plugin, ARM image is published under openebs/velero-plugin-arm64 ([#61](https://github.com/openebs/velero-plugin/pull/61),[@akhilerm](https://github.com/akhilerm))
* Updating alpine image version for velero-plugin to 3.10.4 ([#64](https://github.com/openebs/velero-plugin/pull/64),[@mynktl](https://github.com/mynktl))
* support for local snapshot and restore(in different namespace) ([#53](https://github.com/openebs/velero-plugin/pull/53),[@mynktl](https://github.com/mynktl))
* added support for multiPartChunkSize for S3 based remote storage ([#55](https://github.com/openebs/velero-plugin/pull/55),[@mynktl](https://github.com/mynktl))
* added auto clean-up of CStor volume snapshot generated for remote backup ([#57](https://github.com/openebs/velero-plugin/pull/57),[@mynktl](https://github.com/mynktl))


1.9.0-RC2 / 2020-04-12
========================
* ARM build for velero-plugin, ARM image is published under openebs/velero-plugin-arm64 ([#61](https://github.com/openebs/velero-plugin/pull/61),[@akhilerm](https://github.com/akhilerm))
* Updating alpine image version for velero-plugin to 3.10.4 ([#64](https://github.com/openebs/velero-plugin/pull/64),[@mynktl](https://github.com/mynktl))


1.9.0-RC1 / 2020-04-08
========================
* support for local snapshot and restore(in different namespace) ([#53](https://github.com/openebs/velero-plugin/pull/53),[@mynktl](https://github.com/mynktl))
* added support for multiPartChunkSize for S3 based remote storage ([#55](https://github.com/openebs/velero-plugin/pull/55),[@mynktl](https://github.com/mynktl))
* added auto clean-up of CStor volume snapshot generated for remote backup ([#57](https://github.com/openebs/velero-plugin/pull/57),[@mynktl](https://github.com/mynktl))
