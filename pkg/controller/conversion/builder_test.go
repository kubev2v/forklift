package conversion

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func initBuilderSettings(t *testing.T) {
	t.Helper()
	origCPUReq := Settings.VirtV2vContainerRequestsCpu
	origMemReq := Settings.VirtV2vContainerRequestsMemory
	origCPULim := Settings.VirtV2vContainerLimitsCpu
	origMemLim := Settings.VirtV2vContainerLimitsMemory
	t.Cleanup(func() {
		Settings.VirtV2vContainerRequestsCpu = origCPUReq
		Settings.VirtV2vContainerRequestsMemory = origMemReq
		Settings.VirtV2vContainerLimitsCpu = origCPULim
		Settings.VirtV2vContainerLimitsMemory = origMemLim
	})
	Settings.VirtV2vContainerRequestsCpu = "100m"
	Settings.VirtV2vContainerRequestsMemory = "128Mi"
	Settings.VirtV2vContainerLimitsCpu = "1"
	Settings.VirtV2vContainerLimitsMemory = "512Mi"
}

func minimalSecret() *core.Secret {
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{Name: "v2v-secret", Namespace: "default"},
	}
}

func TestGetVirtV2vPodSpec_VDDKImage_NfcPluginSubPathMount(t *testing.T) {
	initBuilderSettings(t)

	b := &Builder{
		Config: convctx.PodConfig{
			TargetNamespace: "default",
			VDDKImage:       "test-registry.example.com/vddk-test:latest",
		},
	}

	pod, _, err := b.GetVirtV2vPodSpec(
		&plan.VMStatus{},
		nil,
		nil,
		nil,
		minimalSecret(),
		false,
	)
	if err != nil {
		t.Fatalf("GetVirtV2vPodSpec returned error: %v", err)
	}

	mounts := pod.Spec.Containers[0].VolumeMounts
	found := false
	for _, m := range mounts {
		if m.MountPath == nbdkitNfcPluginMountPath && m.SubPath == nbdkitNfcPluginSubPath {
			if m.Name != convctx.VddkVolumeName {
				t.Errorf("NFC plugin mount uses wrong volume name: got %q, want %q", m.Name, convctx.VddkVolumeName)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SubPath mount at %s with subPath %s, but it was not found in volume mounts: %v",
			nbdkitNfcPluginMountPath, nbdkitNfcPluginSubPath, mounts)
	}
}

func TestGetVirtV2vPodSpec_NoVDDKImage_NoNfcPluginMount(t *testing.T) {
	initBuilderSettings(t)

	b := &Builder{
		Config: convctx.PodConfig{
			TargetNamespace: "default",
		},
	}

	pod, _, err := b.GetVirtV2vPodSpec(
		&plan.VMStatus{},
		nil,
		nil,
		nil,
		minimalSecret(),
		false,
	)
	if err != nil {
		t.Fatalf("GetVirtV2vPodSpec returned error: %v", err)
	}

	for _, m := range pod.Spec.Containers[0].VolumeMounts {
		if m.MountPath == nbdkitNfcPluginMountPath {
			t.Errorf("NFC plugin mount at %s should not be present when VDDKImage is empty", nbdkitNfcPluginMountPath)
		}
	}
}

func TestGetVirtV2vPodSpec_VDDKImage_InitContainer(t *testing.T) {
	initBuilderSettings(t)

	vddkImage := "test-registry.example.com/vddk-test:latest"
	b := &Builder{
		Config: convctx.PodConfig{
			TargetNamespace: "default",
			VDDKImage:       vddkImage,
		},
	}

	pod, _, err := b.GetVirtV2vPodSpec(
		&plan.VMStatus{},
		nil,
		nil,
		nil,
		minimalSecret(),
		false,
	)
	if err != nil {
		t.Fatalf("GetVirtV2vPodSpec returned error: %v", err)
	}

	found := false
	for _, c := range pod.Spec.InitContainers {
		if c.Name == "vddk-side-car" && c.Image == vddkImage {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected vddk-side-car init container with image %s", vddkImage)
	}
}
