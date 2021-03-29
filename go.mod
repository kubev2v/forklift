module github.com/konveyor/forklift-controller

go 1.14

require (
	github.com/gin-gonic/gin v1.6.3
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.1.0
	github.com/konveyor/controller v0.3.1
	github.com/onsi/gomega v1.10.3
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/vmware/govmomi v0.23.1
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/client-go v12.0.0+incompatible
	kubevirt.io/client-go v0.33.0
	kubevirt.io/containerized-data-importer v1.27.0
	kubevirt.io/vm-import-operator v0.0.0-00010101000000-000000000000
	sigs.k8s.io/controller-runtime v0.6.4
)

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

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309

replace kubevirt.io/vm-import-operator => github.com/kubevirt/vm-import-operator v0.3.0
