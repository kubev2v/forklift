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

func TestGetNutanixNfsPopulatorVolumes(t *testing.T) {
	tests := []struct {
		name      string
		serverIP  string
		export    string
		wantErr   bool
		wantPath  string
		wantMount string
	}{
		{
			name:      "valid server and export",
			serverIP:  "10.0.0.1",
			export:    "SelfServiceContainer",
			wantPath:  "/SelfServiceContainer",
			wantMount: "/source",
		},
		{
			name:      "export with leading slash is normalized",
			serverIP:  "10.0.0.1",
			export:    "/SelfServiceContainer",
			wantPath:  "/SelfServiceContainer",
			wantMount: "/source",
		},
		{
			name:    "missing server",
			export:  "SelfServiceContainer",
			wantErr: true,
		},
		{
			name:     "missing export",
			serverIP: "10.0.0.1",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			populator := &v1beta1.NutanixNfsVolumePopulator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-nnvp", Namespace: "default"},
				Spec: v1beta1.NutanixNfsVolumePopulatorSpec{
					ServerIP:   tt.serverIP,
					Export:     tt.export,
					SourcePath: ".acropolis/vmdisk/disk-1",
					SecretName: "secret",
				},
			}

			raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(populator)
			if err != nil {
				t.Fatal(err)
			}
			u := &unstructured.Unstructured{Object: raw}

			volumes, mounts, err := getNutanixNfsPopulatorVolumes(u)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if len(volumes) != 1 || len(mounts) != 1 {
				t.Fatalf("expected 1 volume and 1 mount, got %d/%d", len(volumes), len(mounts))
			}
			if volumes[0].NFS == nil {
				t.Fatal("expected NFS volume source")
			}
			if volumes[0].NFS.Server != tt.serverIP {
				t.Errorf("expected server %q, got %q", tt.serverIP, volumes[0].NFS.Server)
			}
			if volumes[0].NFS.Path != tt.wantPath {
				t.Errorf("expected path %q, got %q", tt.wantPath, volumes[0].NFS.Path)
			}
			if !volumes[0].NFS.ReadOnly {
				t.Error("expected read-only NFS volume")
			}
			if mounts[0].MountPath != tt.wantMount {
				t.Errorf("expected mount path %q, got %q", tt.wantMount, mounts[0].MountPath)
			}
			if mounts[0].Name != volumes[0].Name {
				t.Errorf("expected mount name %q to match volume name %q", mounts[0].Name, volumes[0].Name)
			}
		})
	}
}

func TestGetNutanixNfsPopulatorPodArgs(t *testing.T) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pvc", Namespace: "default", UID: "pvc-uid"},
	}
	populator := &v1beta1.NutanixNfsVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nnvp", Namespace: "default"},
		Spec: v1beta1.NutanixNfsVolumePopulatorSpec{
			ServerIP:      "10.0.0.1",
			Export:        "SelfServiceContainer",
			SourcePath:    ".acropolis/vmdisk/disk-1",
			DiskSizeBytes: 1024,
			SecretName:    "secret",
		},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(populator)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: raw}

	args, err := getNutanixNfsPopulatorPodArgs(false, u, pvc)
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--volume-path=/mnt/disk.img",
		"--source-path=.acropolis/vmdisk/disk-1",
		"--secret-name=secret",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in args, got %v", want, args)
		}
	}
	// The NFS endpoint is expressed via the pod volume, not CLI args, and
	// --pvc-size/--owner-uid are appended by the machinery (controller.go).
	for _, notWant := range []string{"--server-ip=", "--export=", "--pvc-size=", "--owner-uid="} {
		if strings.Contains(joined, notWant) {
			t.Errorf("did not expect %q in args, got %v", notWant, args)
		}
	}
}

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
