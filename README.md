# Velero Plugin for OpenEBS ZFS LocalPV

This repository provides a Velero volume snapshotter plugin for backing up and
restoring OpenEBS ZFS LocalPV volumes to AWS S3, Google Cloud Storage, or an
S3-compatible object store.

[![Build Status](https://github.com/openebs/velero-plugin/actions/workflows/build.yml/badge.svg)](https://github.com/openebs/velero-plugin/actions/workflows/build.yml)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)

## Prerequisites

- Velero
- OpenEBS ZFS LocalPV
- An object-storage bucket and credentials available to the Velero plugin

## Install the plugin

```console
velero plugin add openebs/velero-plugin:<VERSION>
```

## Configure the snapshot location

Create a Velero `VolumeSnapshotLocation` using the
`openebs.io/zfspv-blockstore` provider. The `namespace`, `provider`, and
`bucket` values are required; `region` is also required when `provider` is
`aws`.

```yaml
apiVersion: velero.io/v1
kind: VolumeSnapshotLocation
metadata:
  name: openebs-zfs
  namespace: velero
spec:
  provider: openebs.io/zfspv-blockstore
  config:
    namespace: openebs
    provider: aws
    bucket: <BUCKET_NAME>
    region: <AWS_REGION>
```

See [`example/06-volumesnapshotlocation.yaml`](example/06-volumesnapshotlocation.yaml)
for optional S3 and incremental-backup settings.

## Back up and restore

```console
velero backup create zfs-backup \
  --include-namespaces=<NAMESPACE> \
  --snapshot-volumes \
  --volume-snapshot-locations=openebs-zfs

velero restore create --from-backup zfs-backup --restore-volumes=true
```

For scheduled incremental backups, set `incrBackupCount` in the snapshot
location and create a Velero schedule:

```console
velero schedule create zfs-schedule \
  --schedule="0 */6 * * *" \
  --include-namespaces=<NAMESPACE> \
  --snapshot-volumes \
  --volume-snapshot-locations=openebs-zfs
```

## Development

```console
make build
make test
```

Build the container image with:

```console
make container IMAGE=<IMAGE_NAME>
```

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
