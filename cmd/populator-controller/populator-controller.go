package main

import (
	"flag"
	"os"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	populator_machinery "github.com/kubev2v/lib-volume-populator/populator-machinery"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

const (
	prefix     = "forklift.konveyor.io"
	mountPath  = "/mnt/"
	devicePath = "/dev/block"
)

const (
	groupName  = "forklift.konveyor.io"
	apiVersion = "v1beta1"
	kind       = "OvirtVolumePopulator"
	resource   = "ovirtvolumepopulators"
)

func main() {
	var httpEndpoint, metricsPath, masterURL, kubeconfig string

	// Controller args
	if f := flag.Lookup("kubeconfig"); f != nil {
		kubeconfig = f.Value.String()
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	// Metrics args
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")

	flag.Parse()

	namespace := os.Getenv("POD_NAMESPACE")

	if ovirtImage, ok := os.LookupEnv("OVIRT_POPULATOR_IMAGE"); ok {
		klog.Info("Tracking ovirt populators")
		gk := schema.GroupKind{Group: groupName, Kind: kind}
		gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}
		populator_machinery.RunController(masterURL, kubeconfig, ovirtImage, httpEndpoint, metricsPath,
			namespace, prefix, gk, gvr, mountPath, devicePath, getPopulatorPodArgs)
	} else {
		klog.Warning("Couldn't find OVIRT_POPULATOR_IMAGE, don't track ovirt populators")
	}
}

func getPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var ovirtVolumePopulator v1beta1.OvirtVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ovirtVolumePopulator)
	if err != nil {
		return nil, err
	}

	var args []string
	if rawBlock {
		args = append(args, "--volume-path="+devicePath)
	} else {
		args = append(args, "--volume-path="+mountPath+"disk.img")
	}

	args = append(args, "--secret-name="+ovirtVolumePopulator.Spec.EngineSecretName)
	args = append(args, "--disk-id="+ovirtVolumePopulator.Spec.DiskID)
	args = append(args, "--engine-url="+ovirtVolumePopulator.Spec.EngineURL)
	args = append(args, "--cr-name="+ovirtVolumePopulator.Name)
	args = append(args, "--cr-namespace="+ovirtVolumePopulator.Namespace)

	return args, nil
}
