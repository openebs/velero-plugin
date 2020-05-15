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
