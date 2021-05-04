module github.com/openebs/velero-plugin

go 1.13

require (
	cloud.google.com/go v0.58.0 // indirect
	cloud.google.com/go/storage v1.9.0 // indirect
	github.com/Azure/azure-pipeline-go v0.2.2 // indirect
	github.com/Azure/azure-storage-blob-go v0.8.0 // indirect
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/aws/aws-sdk-go v1.31.13
	github.com/coreos/go-etcd v2.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/gogo/protobuf v1.3.0 // indirect
	github.com/google/wire v0.4.0 // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/hashicorp/go-plugin v1.0.1-0.20190610192547-a1bc61569a26 // indirect
	github.com/mattn/go-ieproxy v0.0.1 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.1
	github.com/openebs/api v1.11.1-0.20200629052954-e52e2bcd8339
	github.com/openebs/maya v0.0.0-20200411140727-1c81f9e017b0
	github.com/openebs/zfs-localpv v1.6.1-0.20210501133501-1b30116e5fd6
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1 // indirect
	github.com/sirupsen/logrus v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/ugorji/go v1.1.4 // indirect
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	github.com/vmware-tanzu/velero v1.3.2
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	gocloud.dev v0.15.0
	golang.org/x/tools v0.0.0-20200608174601-1b747fd94509 // indirect
	google.golang.org/api v0.26.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/gengo v0.0.0-20190826232639-a874a240740c // indirect
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a // indirect
	k8s.io/kubernetes v1.15.12 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.15.12
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.15.12
	k8s.io/apimachinery => k8s.io/apimachinery v0.15.13-beta.0
	k8s.io/apiserver => k8s.io/apiserver v0.15.12
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.15.12
	k8s.io/client-go => k8s.io/client-go v0.15.12
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.15.12
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.15.12
	k8s.io/code-generator => k8s.io/code-generator v0.15.13-beta.0
	k8s.io/component-base => k8s.io/component-base v0.15.12
	k8s.io/cri-api => k8s.io/cri-api v0.15.13-beta.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.15.12
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.15.12
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.15.12
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.15.12
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.15.12
	k8s.io/kubectl => k8s.io/kubectl v0.15.13-beta.0
	k8s.io/kubelet => k8s.io/kubelet v0.15.12
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.15.12
	k8s.io/metrics => k8s.io/metrics v0.15.12
	k8s.io/node-api => k8s.io/node-api v0.15.12
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.15.12
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.15.12
	k8s.io/sample-controller => k8s.io/sample-controller v0.15.12
)
