# Velero Mayastor Restore Plugin

[![Build](https://github.com/openebs/velero-plugin/actions/workflows/build-release.yaml/badge.svg)](https://github.com/openebs/velero-plugin/actions/workflows/build-release.yaml)

This Velero plugin for OpenEBS Mayastor ensures that `PersistentVolumeClaim` annotations related to StatefulSets (`openebs.io/stsAffinityGroup`) are updated correctly during Velero restores — especially when restoring into a different namespace.

## ✨ Features

- Updates the `openebs.io/stsAffinityGroup` annotation on PVCs during restore
- Ensures StatefulSet PVC grouping works in multiple namespaces

## 🧭 Project Structure

```
.
├── main.go                           # Entry point that registers the plugin with Velero
├── pkg/
│   └── mayastor/
│       └── plugin/
│           └── restoreplugin.go      # Core restore plugin logic
├── Dockerfile                        # Multi-stage Docker build for plugin image
├── Makefile                          # Build, lint, container build, and utility targets
├── _output/                          # Output binary and build artifacts
├── go.mod / go.sum                   # Go module files
└── README.md                         # This documentation
```

## 🛠️ Building

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Make](https://www.gnu.org/software/make/)
- [Go](https://go.dev/doc/install) 1.23+

### Build Plugin Binary

```sh
make build
```

### Build Docker Image

```sh
make container
```

## 🧹 Linting & Formatting

Run all linters and fail if formatting is required:

```sh
make lint
```

Auto-fix formatting issues:

```sh
make lint-fix
```

## 🧪 Usage with Velero

1. Push the built image to your container registry:

```sh
docker tag openebs/velero-plugin-mayastor:<tag> <your-registry>/velero-plugin-mayastor:<tag>
docker push <your-registry>/velero-plugin-mayastor:<tag>
```

2. Patch Velero deployment to use the plugin:

```yaml
spec:
  template:
    spec:
      initContainers:
      - image: docker.io/openebs/velero-plugin-mayastor:develop
        imagePullPolicy: Always
        name: velero-plugin-mayastor
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /target
          name: plugins
```

## 🔍 How It Works

- On restore, the plugin checks PVCs for the annotation `openebs.io/stsAffinityGroup`.
- If it exists, it updates the namespace portion of the annotation value based on Velero’s restore namespace mapping.
- If the namespace hasn't changed, the plugin logs and skips the update.
- If the annotation `openebs.io/stsAffinityGroup` doesn't exist it skips the update.

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
