package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	prefix                  = "forklift.konveyor.io"
	mountPath               = "/mnt/"
	devicePath              = "/dev/block"
	groupName               = "forklift.konveyor.io"
	apiVersion              = "v1beta1"
	maxPopulatorInFlightEnv = "MAX_POPULATOR_INFLIGHT"
)

type populator struct {
	kind            string
	resource        string
	controllerFunc  func(bool, *unstructured.Unstructured, corev1.PersistentVolumeClaim) ([]string, error)
	volumeFunc      populator_machinery.PodVolumeHook
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
		imageVar:        "VSPHERE_COPY_OFFLOAD_POPULATOR_IMAGE",
		metricsEndpoint: ":8082",
	},
	"nutanix-nfs": {
		kind:            "NutanixNfsVolumePopulator",
		resource:        "nutanixnfsvolumepopulators",
		controllerFunc:  getNutanixNfsPopulatorPodArgs,
		volumeFunc:      getNutanixNfsPopulatorVolumes,
		imageVar:        "NUTANIX_NFS_POPULATOR_IMAGE",
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

	var populatorSettings settings.Populator
	populatorSettings.Load()
	resources := buildResources(&populatorSettings)

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
		maxInFlight := getEnvInt(maxPopulatorInFlightEnv, 20)
		gk := schema.GroupKind{Group: groupName, Kind: populator.kind}
		gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: populator.resource}
		controllerFunc := populator.controllerFunc
		volumeFunc := populator.volumeFunc
		metricsEndpoint := populator.metricsEndpoint
		go func() {
			populator_machinery.RunController(masterURL, kubeconfig, imageName, metricsEndpoint, metricsPath,
				prefix, gk, gvr, mountPath, devicePath, controllerFunc, volumeFunc, resources, maxInFlight)
			<-stop
		}()
	}
	<-stop
}

func getEnvInt(name string, def int) int {
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.Atoi(s)
		if err == nil {
			return parsed
		}
	}
	return def
}

func getOvirtPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
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

func getOpenstackPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, _ corev1.PersistentVolumeClaim) ([]string, error) {
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

func getVXPopulatorPodArgs(_ bool, u *unstructured.Unstructured, pvc corev1.PersistentVolumeClaim) ([]string, error) {
	var xcopy v1beta1.VSphereXcopyVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &xcopy)
	if nil != err {
		return nil, err
	}
	args := []string{
		"--source-vm-id=" + xcopy.Spec.VmId,
		"--source-vmdk=" + xcopy.Spec.VmdkPath,
		"--target-namespace=" + xcopy.GetNamespace(),
		"--cr-name=" + xcopy.Name,
		"--cr-namespace=" + xcopy.Namespace,
		"--owner-name=" + pvc.Name,
		"--secret-name=" + xcopy.Spec.SecretName,
		"--storage-vendor-product=" + xcopy.Spec.StorageVendorProduct,
	}
	if xcopy.Spec.MigrationHost != "" {
		args = append(args, "--migration-host="+xcopy.Spec.MigrationHost)
	}
	return args, nil
}

// nfsSourceMountPath is where the populator pod mounts the Nutanix storage
// container export (in-tree nfs volume, read-only, attached by kubelet).
const nfsSourceMountPath = "/source"

func getNutanixNfsPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured, pvc corev1.PersistentVolumeClaim) ([]string, error) {
	var populator v1beta1.NutanixNfsVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &populator)
	if err != nil {
		return nil, err
	}
	args := []string{
		"--volume-path=" + getVolumePath(rawBlock),
		// SourcePath is resolved within the NFS mount attached by kubelet
		// (see getNutanixNfsPopulatorVolumes); the binary never speaks NFS.
		"--source-path=" + populator.Spec.SourcePath,
		"--cr-name=" + populator.Name,
		"--cr-namespace=" + populator.Namespace,
		// The machinery reads --secret-name to wire EnvFrom + cacert into the
		// copy pod (controller.go). Must be non-empty or the pod is invalid.
		"--secret-name=" + populator.Spec.SecretName,
	}
	return args, nil
}

// getNutanixNfsPopulatorVolumes attaches the Nutanix storage container export
// to the populator pod as a read-only in-tree nfs volume. kubelet performs the
// kernel mount on the node; the populator binary reads the vmdisk as a local
// file under nfsSourceMountPath, so qemu-img needs no libnfs nfs:// driver
// (which neither UBI nor RHEL qemu-kvm ships).
func getNutanixNfsPopulatorVolumes(u *unstructured.Unstructured) ([]corev1.Volume, []corev1.VolumeMount, error) {
	var populator v1beta1.NutanixNfsVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &populator)
	if err != nil {
		return nil, nil, err
	}
	if populator.Spec.ServerIP == "" || populator.Spec.Export == "" {
		return nil, nil, fmt.Errorf("populator %s/%s has no NFS server/export", populator.Namespace, populator.Name)
	}

	volume := corev1.Volume{
		Name: "nfs-source",
		VolumeSource: corev1.VolumeSource{
			NFS: &corev1.NFSVolumeSource{
				Server:   populator.Spec.ServerIP,
				Path:     "/" + strings.TrimPrefix(populator.Spec.Export, "/"),
				ReadOnly: true,
			},
		},
	}
	mount := corev1.VolumeMount{
		Name:      volume.Name,
		MountPath: nfsSourceMountPath,
		ReadOnly:  true,
	}
	return []corev1.Volume{volume}, []corev1.VolumeMount{mount}, nil
}

func getVolumePath(rawBlock bool) string {
	if rawBlock {
		return devicePath
	} else {
		return mountPath + "disk.img"
	}
}

func buildResources(s *settings.Populator) *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    s.ContainerLimitsCpu,
			corev1.ResourceMemory: s.ContainerLimitsMemory,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    s.ContainerRequestsCpu,
			corev1.ResourceMemory: s.ContainerRequestsMemory,
		},
	}
}
