# Velero Plugin for OpenEBS

This repository provides a single Velero plugin image for OpenEBS containing:

- `openebs.io/zfspv-blockstore`: a volume snapshotter for backing up and
  restoring OpenEBS ZFS LocalPV volumes to AWS S3, Google Cloud Storage, or an
  S3-compatible object store.
- `openebs.io/velero-plugin-mayastor`: a restore item action for OpenEBS
  Mayastor that rewrites the `openebs.io/stsAffinityGroup` PVC annotation
  when restoring into a different namespace.

[![Build Status](https://github.com/openebs/velero-plugin/actions/workflows/build.yml/badge.svg)](https://github.com/openebs/velero-plugin/actions/workflows/build.yml)
[![Go Report](https://goreportcard.com/badge/github.com/openebs/velero-plugin)](https://goreportcard.com/report/github.com/openebs/velero-plugin)

## Prerequisites

- Velero
- OpenEBS ZFS LocalPV and/or OpenEBS Mayastor
- For ZFS LocalPV: an object-storage bucket and credentials available to the
  Velero plugin

## Install the plugin

```console
velero plugin add openebs/velero-plugin:<VERSION>
```

Both plugins are registered by the same binary; the Mayastor restore item
action is applied automatically to `PersistentVolumeClaim` resources during
restore and needs no configuration.

## ZFS LocalPV

### Configure the snapshot location

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

### Back up and restore

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

## Mayastor

Mayastor groups the PVCs of a StatefulSet using the
`openebs.io/stsAffinityGroup: <namespace>/<group>` annotation. When a backup is
restored with a Velero namespace mapping, the plugin updates the namespace part
of the annotation to the target namespace so the affinity group remains valid.
PVCs without the annotation are left untouched.

```console
velero restore create --from-backup <BACKUP> --namespace-mappings <SRC_NS>:<DST_NS>
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
