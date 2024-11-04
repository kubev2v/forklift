package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	populator_machinery "github.com/konveyor/forklift-controller/pkg/lib-volume-populator/populator-machinery"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
)

const (
	prefix     = "forklift.konveyor.io"
	mountPath  = "/mnt/"
	devicePath = "/dev/block"
	groupName  = "forklift.konveyor.io"
	apiVersion = "v1beta1"
)

type populator struct {
	kind            string
	resource        string
	controllerFunc  func(bool, *unstructured.Unstructured) ([]string, error)
	imageVar        string
	metricsEndpoint string
}

var populators = map[string]populator{
	"ovirt": {
		kind:            "OvirtVolumePopulator",
		resource:        "ovirtvolumepopulators",
		controllerFunc:  getOvirtPopulatorPodArgs,
		imageVar:        "OVIRT_POPULATOR_IMAGE",
		metricsEndpoint: ":8080",
	},
	"openstack": {
		kind:            "OpenstackVolumePopulator",
		resource:        "openstackvolumepopulators",
		controllerFunc:  getOpenstackPopulatorPodArgs,
		imageVar:        "OPENSTACK_POPULATOR_IMAGE",
		metricsEndpoint: ":8081",
	},
	"offloadPlugin": {
		kind:            "OffloadPluginVolumePopulator",
		resource:        "offloadpluginvolumepopulators",
		controllerFunc:  getOffloadPluginPopulatorPodArgs,
		imageVar:        "", // the imageVar is empty so the volume populator controller gets the image from the resource specification
		metricsEndpoint: ":8082",
	},
}

func main() {
	var metricsPath, masterURL, kubeconfig string

	// Controller args
	if f := flag.Lookup("kubeconfig"); f != nil {
		kubeconfig = f.Value.String()
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	// Metrics args
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan bool)
	go func() {
		<-sigs
		stop <- true
	}()

	for _, populator := range populators {
		var imageName string
		if populator.imageVar != "" {
			var ok bool
			imageName, ok = os.LookupEnv(populator.imageVar)
			if !ok {
				klog.Warning("Couldn't find", "imageVar", populator.imageVar)
				continue
			}
		}
		gk := schema.GroupKind{Group: groupName, Kind: populator.kind}
		gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: populator.resource}
		controllerFunc := populator.controllerFunc
		metricsEndpoint := populator.metricsEndpoint
		go func() {
			populator_machinery.RunController(masterURL, kubeconfig, imageName, metricsEndpoint, metricsPath,
				prefix, gk, gvr, mountPath, devicePath, controllerFunc)
			<-stop
		}()
	}
	<-stop
}

func getOvirtPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var ovirtVolumePopulator v1beta1.OvirtVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ovirtVolumePopulator)
	if err != nil {
		return nil, err
	}

	var args []string
	args = append(args, "--volume-path="+getVolumePath(rawBlock))
	args = append(args, "--secret-name="+ovirtVolumePopulator.Spec.EngineSecretName)
	args = append(args, "--disk-id="+ovirtVolumePopulator.Spec.DiskID)
	args = append(args, "--engine-url="+ovirtVolumePopulator.Spec.EngineURL)
	args = append(args, "--cr-name="+ovirtVolumePopulator.Name)
	args = append(args, "--cr-namespace="+ovirtVolumePopulator.Namespace)

	return args, nil
}

func getOpenstackPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var openstackPopulator v1beta1.OpenstackVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &openstackPopulator)
	if nil != err {
		return nil, err
	}
	args := []string{}
	args = append(args, "--volume-path="+getVolumePath(rawBlock))
	args = append(args, "--endpoint="+openstackPopulator.Spec.IdentityURL)
	args = append(args, "--secret-name="+openstackPopulator.Spec.SecretName)
	args = append(args, "--image-id="+openstackPopulator.Spec.ImageID)
	args = append(args, "--cr-name="+openstackPopulator.Name)
	args = append(args, "--cr-namespace="+openstackPopulator.Namespace)

	return args, nil
}

func getOffloadPluginPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var offloadPlugin v1beta1.OffloadPluginVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &offloadPlugin)
	if err != nil {
		return nil, err
	}
	args := []string{}
	args = append(args, "--volume-path="+getVolumePath(rawBlock))
	args = append(args, "--cr-name="+offloadPlugin.Name)
	args = append(args, "--cr-namespace="+offloadPlugin.Namespace)
	args = append(args, "--secret-name="+offloadPlugin.Spec.SecretName)

	return args, nil
}

func getVolumePath(rawBlock bool) string {
	if rawBlock {
		return devicePath
	} else {
		return mountPath + "disk.img"
	}
}
