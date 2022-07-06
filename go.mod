module github.com/konveyor/forklift-controller

go 1.14

require (
	github.com/gin-gonic/gin v1.7.2
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-logr/logr v0.3.0
	github.com/go-openapi/spec v0.19.4 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.0
	github.com/konveyor/controller v0.10.0
	github.com/onsi/gomega v1.10.3
	github.com/openshift/api v0.0.0
	github.com/openshift/library-go v0.0.0-20200821154433-215f00df72cc
	github.com/ovirt/go-ovirt v4.3.4+incompatible
	github.com/pkg/profile v1.3.0
	github.com/prometheus/client_golang v1.11.0
	github.com/vmware/govmomi v0.23.1
	go.uber.org/zap v1.14.1 // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/apiserver v0.20.2
	k8s.io/client-go v12.0.0+incompatible
	kubevirt.io/client-go v0.42.1
	kubevirt.io/containerized-data-importer-api v1.44.0
	libvirt.org/libvirt-go-xml v6.6.0+incompatible
	sigs.k8s.io/controller-runtime v0.8.3
)

// CVE-2021-41190
replace github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2

replace bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c => github.com/markusthoemmes/goautoneg v0.0.0-20190713162725-c6008fefa5b1

//openshift deps pinning
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190716152234-9ea19f9dd578

replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191125132246-f6563a70e19a

// k8s deps pinning
replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4

replace k8s.io/api => k8s.io/api v0.19.3

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.3

replace k8s.io/apimachinery => k8s.io/apimachinery v0.19.3

replace k8s.io/apiserver => k8s.io/apiserver v0.19.3

replace k8s.io/client-go => k8s.io/client-go v0.19.3

replace k8s.io/code-generator => k8s.io/code-generator v0.19.3

replace k8s.io/component-base => k8s.io/component-base v0.19.3

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.3

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.3

replace k8s.io/kubectl => k8s.io/kubectl v0.19.3

replace k8s.io/kubernetes => k8s.io/kubernetes v0.19.3

replace sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v1.0.1-0.20191108220359-b1b620dd3f06

replace github.com/ovirt/go-ovirt => github.com/ovirt/go-ovirt v0.0.0-20210423075620-0fe653f1c0cd

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.6.4
