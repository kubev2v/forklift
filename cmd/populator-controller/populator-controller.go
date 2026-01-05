package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	populator_machinery "github.com/kubev2v/forklift/pkg/lib-volume-populator/populator-machinery"
	"github.com/kubev2v/forklift/pkg/settings"
	corev1 "k8s.io/api/core/v1"
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
	controllerFunc  func(bool, *unstructured.Unstructured, corev1.PersistentVolumeClaim) ([]string, error)
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
	"vsphere-xcopy": {
		kind:            "VSphereXcopyVolumePopulator",
		resource:        "vspherexcopyvolumepopulators",
		controllerFunc:  getVXPopulatorPodArgs,
		imageVar:        "VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE",
		metricsEndpoint: ":8082",
	},
	"ec2": {
		kind:            "Ec2VolumePopulator",
		resource:        "ec2volumepopulators",
		controllerFunc:  getEc2PopulatorPodArgs,
		imageVar:        "EC2_POPULATOR_IMAGE",
		metricsEndpoint: ":8083",
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
	klog.InitFlags(nil)
	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan bool)
	go func() {
		<-sigs
		stop <- true
	}()

	for _, populator := range populators {
		imageName, ok := os.LookupEnv(populator.imageVar)
		if !ok {
			klog.Warning("Couldn't find", "imageVar", populator.imageVar)
			continue
		}
		gk := schema.GroupKind{Group: groupName, Kind: populator.kind}
		gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: populator.resource}
		controllerFunc := populator.controllerFunc
		metricsEndpoint := populator.metricsEndpoint
		go func() {
			populator_machinery.RunController(masterURL, kubeconfig, imageName, metricsEndpoint, metricsPath,
				prefix, gk, gvr, mountPath, devicePath, controllerFunc, resources)
			<-stop
		}()
	}
	<-stop
}

func getOvirtPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
	var ovirtVolumePopulator v1beta1.OvirtVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ovirtVolumePopulator)
	if err != nil {
		return nil, err
	}

	return []string{
		"--volume-path=" + getVolumePath(rawBlock),
		"--secret-name=" + ovirtVolumePopulator.Spec.EngineSecretName,
		"--disk-id=" + ovirtVolumePopulator.Spec.DiskID,
		"--engine-url=" + ovirtVolumePopulator.Spec.EngineURL,
		"--cr-name=" + ovirtVolumePopulator.Name,
		"--cr-namespace=" + ovirtVolumePopulator.Namespace,
	}, nil
}

func getOpenstackPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
	var openstackPopulator v1beta1.OpenstackVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &openstackPopulator)
	if nil != err {
		return nil, err
	}

	return []string{
		"--volume-path=" + getVolumePath(rawBlock),
		"--endpoint=" + openstackPopulator.Spec.IdentityURL,
		"--secret-name=" + openstackPopulator.Spec.SecretName,
		"--image-id=" + openstackPopulator.Spec.ImageID,
		"--cr-name=" + openstackPopulator.Name,
		"--cr-namespace=" + openstackPopulator.Namespace,
	}, nil
}

func getVXPopulatorPodArgs(_ bool, u *unstructured.Unstructured, pvc corev1.PersistentVolumeClaim) ([]string, error) {
	var xcopy v1beta1.VSphereXcopyVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &xcopy)
	if nil != err {
		return nil, err
	}

	return []string{
		"--source-vm-id=" + xcopy.Spec.VmId,
		"--source-vmdk=" + xcopy.Spec.VmdkPath,
		"--target-namespace=" + xcopy.GetNamespace(),
		"--cr-name=" + xcopy.Name,
		"--cr-namespace=" + xcopy.Namespace,
		"--owner-name=" + pvc.Name,
		"--secret-name=" + xcopy.Spec.SecretName,
		"--storage-vendor-product=" + xcopy.Spec.StorageVendorProduct,
	}, nil
}

func getEc2PopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
	var ec2Populator v1beta1.Ec2VolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ec2Populator)
	if err != nil {
		return nil, err
	}

	// EC2 populator creates PVs via AWS API, doesn't need volume-path
	// (unlike oVirt/OpenStack populators that copy data to mounted volumes)
	return []string{
		"--region=" + ec2Populator.Spec.Region,
		"--target-availability-zone=" + ec2Populator.Spec.TargetAvailabilityZone,
		"--secret-name=" + ec2Populator.Spec.SecretName,
		"--snapshot-id=" + ec2Populator.Spec.SnapshotID,
		"--cr-name=" + ec2Populator.Name,
		"--cr-namespace=" + ec2Populator.Namespace,
	}, nil
}

func getVolumePath(rawBlock bool) string {
	if rawBlock {
		return devicePath
	} else {
		return mountPath + "disk.img"
	}
}

func getResources() (*corev1.ResourceRequirements, error) {
	cpuLimit := settings.Settings.Migration.PopulatorContainerLimitsCpu
	memoryLimit := settings.Settings.Migration.PopulatorContainerLimitsMemory
	cpuRequest := settings.Settings.Migration.PopulatorContainerRequestsCpu
	memoryRequest := settings.Settings.Migration.PopulatorContainerRequestsMemory
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    cpuLimit,
			corev1.ResourceMemory: memoryLimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
	}, nil
}
