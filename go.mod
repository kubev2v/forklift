module github.com/konveyor/forklift-controller

go 1.18

require (
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.7.2
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zapr v0.4.0
	github.com/google/uuid v1.1.2
	github.com/gophercloud/gophercloud v1.3.0
	github.com/gophercloud/utils v0.0.0-20230418172808-6eab72e966e1
	github.com/gorilla/websocket v1.5.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.0
	github.com/mattn/go-sqlite3 v1.14.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.19.0
	github.com/openshift/api v0.0.0
	github.com/openshift/library-go v0.0.0-20200821154433-215f00df72cc
	github.com/ovirt/go-ovirt v4.3.4+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.3.0
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/client_model v0.3.0
	github.com/prometheus/common v0.39.0
	github.com/vmware/govmomi v0.23.1
	go.uber.org/zap v1.19.0
	golang.org/x/net v0.4.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.26.1
	k8s.io/apiextensions-apiserver v0.23.5
	k8s.io/apimachinery v0.26.1
	k8s.io/apiserver v0.20.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/component-base v0.26.0
	k8s.io/component-helpers v0.26.0
	k8s.io/klog/v2 v2.80.1
	kubevirt.io/containerized-data-importer-api v1.56.0
	libvirt.org/libvirt-go-xml v6.6.0+incompatible
	sigs.k8s.io/controller-runtime v0.8.3
)

require github.com/elazarl/goproxy v0.0.0-20190911111923-ecfe977594f1 // indirect

require (
	cloud.google.com/go v0.65.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/go-playground/validator/v10 v10.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/openshift/custom-resource-status v1.1.2 // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/ugorji/go/codec v1.1.7 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/oauth2 v0.3.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/term v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	golang.org/x/time v0.1.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
	kubevirt.io/api v0.59.1
	kubevirt.io/controller-lifecycle-operator-sdk/api v0.0.0-20220329064328-f3cc58c6ed90 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// CVE-2021-41190
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2

replace bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c => github.com/markusthoemmes/goautoneg v0.0.0-20190713162725-c6008fefa5b1

//openshift deps pinning
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230406152840-ce21e3fe5da2

replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191125132246-f6563a70e19a

// k8s deps pinning
replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4

replace k8s.io/api => k8s.io/api v0.22.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3

replace k8s.io/apimachinery => k8s.io/apimachinery v0.22.0

replace k8s.io/apiserver => k8s.io/apiserver v0.19.3

replace k8s.io/client-go => k8s.io/client-go v0.22.0

replace k8s.io/code-generator => k8s.io/code-generator v0.19.3

replace k8s.io/component-base => k8s.io/component-base v0.19.3

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.3

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.3

replace k8s.io/kubectl => k8s.io/kubectl v0.19.3

replace k8s.io/kubernetes => k8s.io/kubernetes v0.19.3

replace sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06

replace github.com/ovirt/go-ovirt => github.com/ovirt/go-ovirt v0.0.0-20210423075620-0fe653f1c0cd

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.10.0

replace github.com/go-logr/logr => github.com/go-logr/logr v0.4.0

replace k8s.io/klog/v2 => k8s.io/klog/v2 v2.5.0

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd

replace github.com/gophercloud/gophercloud => github.com/kubev2v/gophercloud v0.0.0-20230629135522-9d701a75c760
