package main

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func TestGetVXPopulatorPodArgs_MigrationHost(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default"},
	}

	tests := []struct {
		name          string
		migrationHost string
		wantArg       bool
	}{
		{
			name:          "migration host set",
			migrationHost: "host-36",
			wantArg:       true,
		},
		{
			name:          "migration host empty",
			migrationHost: "",
			wantArg:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xcopy := &v1beta1.VSphereXcopyVolumePopulator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-xcopy", Namespace: "default"},
				Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
					VmId:                 "vm-123",
					VmdkPath:             "[ds] vm/disk.vmdk",
					SecretName:           "secret",
					StorageVendorProduct: "ontap",
					MigrationHost:        tt.migrationHost,
				},
			}

			raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(xcopy)
			if err != nil {
				t.Fatal(err)
			}
			u := &unstructured.Unstructured{Object: raw}

			args, err := getVXPopulatorPodArgs(false, u, pvc)
			if err != nil {
				t.Fatal(err)
			}

			found := false
			for _, arg := range args {
				if arg == "--migration-host="+tt.migrationHost {
					found = true
				}
			}

			if tt.wantArg && !found {
				t.Errorf("expected --migration-host=%s in args, got %v", tt.migrationHost, args)
			}
			if !tt.wantArg && found {
				t.Errorf("did not expect --migration-host in args, got %v", args)
			}
		})
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestGetVNPopulatorPodArgs_NfsCopy(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default"},
	}
	nfsCopy := &v1beta1.VSphereNfsCopyVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nfscopy", Namespace: "default"},
		Spec: v1beta1.VSphereNfsCopyVolumePopulatorSpec{
			VmId:                 "vm-123",
			VmdkPath:             "[ds] vm/disk.vmdk",
			SecretName:           "secret",
			StorageVendorProduct: "pureFlashArray",
		},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(nfsCopy)
	if err != nil {
		t.Fatal(err)
	}
	args, err := getVNPopulatorPodArgs(false, &unstructured.Unstructured{Object: raw}, pvc)
	if err != nil {
		t.Fatal(err)
	}

	if !hasArg(args, "--nfs-copy") {
		t.Errorf("--nfs-copy missing, args %v", args)
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--migration-host=") {
			t.Errorf("NFS copy must not set a migration host, args %v", args)
		}
	}
	if !hasArg(args, "--source-vm-id=vm-123") || !hasArg(args, "--storage-vendor-product=pureFlashArray") {
		t.Errorf("spec not carried into args, args %v", args)
	}
}

func TestGetVXPopulatorPodArgs_NeverRequestsNfsCopy(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default"},
	}
	xcopy := &v1beta1.VSphereXcopyVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{Name: "test-xcopy", Namespace: "default"},
		Spec: v1beta1.VSphereXcopyVolumePopulatorSpec{
			VmId:                 "vm-123",
			VmdkPath:             "[ds] vm/disk.vmdk",
			SecretName:           "secret",
			StorageVendorProduct: "pureFlashArray",
		},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(xcopy)
	if err != nil {
		t.Fatal(err)
	}
	args, err := getVXPopulatorPodArgs(false, &unstructured.Unstructured{Object: raw}, pvc)
	if err != nil {
		t.Fatal(err)
	}

	if hasArg(args, "--nfs-copy") {
		t.Errorf("xcopy populator must never request an NFS copy, args %v", args)
	}
}
