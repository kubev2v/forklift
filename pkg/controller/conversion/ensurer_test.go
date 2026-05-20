package conversion

import (
	"context"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testEnsurer(t *testing.T, objs ...runtime.Object) *Ensurer {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme core: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &Ensurer{
		Client:            cl,
		DestinationClient: cl,
		Log:               logging.WithName("test"),
	}
}

func pvcWithMode(name, ns string, mode core.PersistentVolumeMode) *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{Name: name, Namespace: ns},
		Spec:       core.PersistentVolumeClaimSpec{VolumeMode: &mode},
	}
}

func pvcFilesystem(name, ns string) *core.PersistentVolumeClaim {
	return pvcWithMode(name, ns, core.PersistentVolumeFilesystem)
}

func pvcBlock(name, ns string) *core.PersistentVolumeClaim {
	return pvcWithMode(name, ns, core.PersistentVolumeBlock)
}

// conversionWithSnapshot builds a minimal DeepInspection Conversion whose
// controller-owned snapshot has the given MoRef. Pass moref="" to simulate a
// state where snapshot creation has not yet completed.
func conversionWithSnapshot(moref string) *api.Conversion {
	conv := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
		Spec: api.ConversionSpec{
			Type: api.DeepInspection,
			// No SNAPSHOT_MOREF in Settings → controller owns the snapshot.
			Connection: api.Connection{
				Secret: core.ObjectReference{
					Name:      "vsphere-secret",
					Namespace: "default",
				},
			},
		},
		Status: api.ConversionStatus{
			Snapshot: &api.SnapshotStatus{
				Owned: true,
				Moref: moref,
			},
		},
	}
	return conv
}

// TestRemoveOwnedSnapshot_Guards verifies that RemoveOwnedSnapshot returns
// (true, nil) immediately — without contacting vSphere — for all cases where
// there is nothing to clean up. This is the guard logic that protects the
// happy-path (SNAPSHOT_MOREF supplied by caller, non-DeepInspection types, or
// snapshot not yet created) from triggering an unnecessary removal.
func TestRemoveOwnedSnapshot_Guards(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		conv     *api.Conversion
		wantDone bool
		wantErr  bool
	}{
		{
			name: "non-DeepInspection type is a no-op",
			conv: &api.Conversion{
				Spec:   api.ConversionSpec{Type: api.Remote},
				Status: api.ConversionStatus{Snapshot: &api.SnapshotStatus{Moref: "snapshot-1", Owned: true}},
			},
			wantDone: true,
		},
		{
			// SNAPSHOT_MOREF in settings means the caller owns the snapshot;
			// Status.Snapshot.Owned is false as runPhasePending would set it.
			name: "snapshot not owned by controller (SNAPSHOT_MOREF was supplied)",
			conv: &api.Conversion{
				ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
				Spec: api.ConversionSpec{
					Type:     api.DeepInspection,
					Settings: map[string]string{api.SpecSettingsSnapshotMorefKey: "snapshot-2"},
				},
				Status: api.ConversionStatus{
					Snapshot: &api.SnapshotStatus{Owned: false, Moref: "snapshot-2"},
				},
			},
			wantDone: true,
		},
		{
			name:     "snapshot status is nil",
			conv:     &api.Conversion{Spec: api.ConversionSpec{Type: api.DeepInspection}},
			wantDone: true,
		},
		{
			name: "snapshot moref is empty (creation not yet completed)",
			conv: conversionWithSnapshot(""),
			// moref="" → nothing to remove yet
			wantDone: true,
		},
		{
			name: "moref set but connection secret name is empty",
			conv: func() *api.Conversion {
				c := conversionWithSnapshot("snapshot-3")
				c.Spec.Connection.Secret.Name = ""
				return c
			}(),
			// must not attempt vSphere; returns error to surface misconfiguration
			wantDone: false,
			wantErr:  true,
		},
		{
			name: "moref set but connection secret namespace is empty",
			conv: func() *api.Conversion {
				c := conversionWithSnapshot("snapshot-4")
				c.Spec.Connection.Secret.Namespace = ""
				return c
			}(),
			wantDone: false,
			wantErr:  true,
		},
		{
			// Secret ref is fully set but the secret does not exist in the cluster.
			// testEnsurer uses an empty fake client so e.Client.Get returns not-found.
			name:     "moref set, valid secret ref, but secret not found in cluster",
			conv:     conversionWithSnapshot("snapshot-5"),
			wantDone: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testEnsurer(t) // no pre-existing k8s objects needed for guard cases
			done, err := e.RemoveOwnedSnapshot(ctx, tt.conv)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if done != tt.wantDone {
				t.Errorf("done = %v, want %v", done, tt.wantDone)
			}
		})
	}
}

// TestSnapshotOwnedByController checks the predicate that decides whether the
// controller created (and is responsible for removing) the snapshot.
func TestSnapshotOwnedByController(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		want     bool
	}{
		{"no settings map", nil, true},
		{"empty SNAPSHOT_MOREF", map[string]string{api.SpecSettingsSnapshotMorefKey: ""}, true},
		{"whitespace-only SNAPSHOT_MOREF", map[string]string{api.SpecSettingsSnapshotMorefKey: "   "}, true},
		{"SNAPSHOT_MOREF provided", map[string]string{api.SpecSettingsSnapshotMorefKey: "snapshot-42"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := &api.Conversion{Spec: api.ConversionSpec{Settings: tt.settings}}
			if got := snapshotOwnedByController(conv); got != tt.want {
				t.Errorf("snapshotOwnedByController = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVolumesFromDiskRefs(t *testing.T) {
	const ns = "test-ns"
	block := core.PersistentVolumeBlock
	fs := core.PersistentVolumeFilesystem

	tests := []struct {
		name         string
		pvcs         []runtime.Object
		disks        []api.DiskRef
		wantMounts   []core.VolumeMount
		wantDevices  []core.VolumeDevice
		wantVolCount int
		wantErr      bool
	}{
		{
			name: "filesystem defaults when MountPath is empty",
			pvcs: []runtime.Object{pvcFilesystem("disk-a", ns)},
			disks: []api.DiskRef{
				{Name: "disk-a", Namespace: ns},
			},
			wantMounts:   []core.VolumeMount{{Name: "disk-a", MountPath: "/mnt/disks/disk0"}},
			wantVolCount: 1,
		},
		{
			name: "block defaults when DevicePath is empty",
			pvcs: []runtime.Object{pvcBlock("disk-b", ns)},
			disks: []api.DiskRef{
				{Name: "disk-b", Namespace: ns, VolumeMode: &block},
			},
			wantDevices:  []core.VolumeDevice{{Name: "disk-b", DevicePath: "/dev/block0"}},
			wantVolCount: 1,
		},
		{
			name: "HyperV provider mount path honored",
			pvcs: []runtime.Object{pvcFilesystem("win-2019-vhdx", ns)},
			disks: []api.DiskRef{
				{Name: "win-2019-vhdx", Namespace: ns, MountPath: "/hyperv/win-2019.vhdx"},
			},
			wantMounts:   []core.VolumeMount{{Name: "win-2019-vhdx", MountPath: "/hyperv/win-2019.vhdx"}},
			wantVolCount: 1,
		},
		{
			name: "OVA provider mount path honored",
			pvcs: []runtime.Object{pvcFilesystem("ova-disk0", ns)},
			disks: []api.DiskRef{
				{Name: "ova-disk0", Namespace: ns, MountPath: "/ova/disk0.vmdk"},
			},
			wantMounts:   []core.VolumeMount{{Name: "ova-disk0", MountPath: "/ova/disk0.vmdk"}},
			wantVolCount: 1,
		},
		{
			name: "custom DevicePath honored for block device",
			pvcs: []runtime.Object{pvcBlock("blk-disk", ns)},
			disks: []api.DiskRef{
				{Name: "blk-disk", Namespace: ns, VolumeMode: &block, DevicePath: "/dev/custom0"},
			},
			wantDevices:  []core.VolumeDevice{{Name: "blk-disk", DevicePath: "/dev/custom0"}},
			wantVolCount: 1,
		},
		{
			name: "mixed disks with and without explicit paths",
			pvcs: []runtime.Object{
				pvcFilesystem("disk0", ns),
				pvcBlock("disk1", ns),
				pvcFilesystem("disk2", ns),
			},
			disks: []api.DiskRef{
				{Name: "disk0", Namespace: ns, MountPath: "/hyperv/disk0.vhdx"},
				{Name: "disk1", Namespace: ns, VolumeMode: &block},
				{Name: "disk2", Namespace: ns},
			},
			wantMounts: []core.VolumeMount{
				{Name: "disk0", MountPath: "/hyperv/disk0.vhdx"},
				{Name: "disk2", MountPath: "/mnt/disks/disk2"},
			},
			wantDevices:  []core.VolumeDevice{{Name: "disk1", DevicePath: "/dev/block1"}},
			wantVolCount: 3,
		},
		{
			name: "VolumeMode from PVC used when DiskRef has none",
			pvcs: []runtime.Object{pvcBlock("pvc-block", ns)},
			disks: []api.DiskRef{
				{Name: "pvc-block", Namespace: ns, DevicePath: "/dev/mydev"},
			},
			wantDevices:  []core.VolumeDevice{{Name: "pvc-block", DevicePath: "/dev/mydev"}},
			wantVolCount: 1,
		},
		{
			name: "DiskRef VolumeMode overrides PVC VolumeMode",
			pvcs: []runtime.Object{pvcBlock("disk-fs", ns)},
			disks: []api.DiskRef{
				{Name: "disk-fs", Namespace: ns, VolumeMode: &fs, MountPath: "/custom/mount"},
			},
			wantMounts:   []core.VolumeMount{{Name: "disk-fs", MountPath: "/custom/mount"}},
			wantVolCount: 1,
		},
		{
			name:    "missing namespace returns error",
			pvcs:    []runtime.Object{},
			disks:   []api.DiskRef{{Name: "no-ns"}},
			wantErr: true,
		},
		{
			name:    "missing PVC returns error",
			pvcs:    []runtime.Object{},
			disks:   []api.DiskRef{{Name: "gone", Namespace: ns}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testEnsurer(t, tt.pvcs...)
			volumes, mounts, devices, err := e.VolumesFromDiskRefs(tt.disks)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := len(volumes); got != tt.wantVolCount {
				t.Errorf("volumes count = %d, want %d", got, tt.wantVolCount)
			}

			if len(mounts) != len(tt.wantMounts) {
				t.Fatalf("mounts count = %d, want %d", len(mounts), len(tt.wantMounts))
			}
			for i, want := range tt.wantMounts {
				if mounts[i].Name != want.Name {
					t.Errorf("mount[%d].Name = %q, want %q", i, mounts[i].Name, want.Name)
				}
				if mounts[i].MountPath != want.MountPath {
					t.Errorf("mount[%d].MountPath = %q, want %q", i, mounts[i].MountPath, want.MountPath)
				}
			}

			if len(devices) != len(tt.wantDevices) {
				t.Fatalf("devices count = %d, want %d", len(devices), len(tt.wantDevices))
			}
			for i, want := range tt.wantDevices {
				if devices[i].Name != want.Name {
					t.Errorf("device[%d].Name = %q, want %q", i, devices[i].Name, want.Name)
				}
				if devices[i].DevicePath != want.DevicePath {
					t.Errorf("device[%d].DevicePath = %q, want %q", i, devices[i].DevicePath, want.DevicePath)
				}
			}
		})
	}
}
