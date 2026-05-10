package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetHyperVPopulatorPodArgs(t *testing.T) {
	cr := map[string]interface{}{
		"apiVersion": "forklift.konveyor.io/v1beta1",
		"kind":       "HyperVVolumePopulator",
		"metadata": map[string]interface{}{
			"name":      "test-populator",
			"namespace": "test-ns",
		},
		"spec": map[string]interface{}{
			"secretName":   "my-secret",
			"vmId":         "vm-123",
			"vmName":       "TestVM",
			"diskIndex":    int64(0),
			"diskPath":     `C:\VMs\TestVM\disk0.vhdx`,
			"targetIQN":    "iqn.2026-03.io.forklift:vm-123",
			"targetPortal": "10.0.0.100:3260",
			"lunId":        int64(0),
			"initiatorIQN": "iqn.2026-03.io.forklift:copy-migration-uid",
		},
	}

	u := &unstructured.Unstructured{Object: cr}
	pvc := corev1.PersistentVolumeClaim{}

	args, err := getHyperVPopulatorPodArgs(true, u, pvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertArgPresent(t, args, "--portal=", "10.0.0.100:3260")
	assertArgPresent(t, args, "--target-iqn=", "iqn.2026-03.io.forklift:vm-123")
	assertArgPresent(t, args, "--initiator-iqn=", "iqn.2026-03.io.forklift:copy-migration-uid")
	assertArgPresent(t, args, "--cr-name=", "test-populator")
	assertArgPresent(t, args, "--cr-namespace=", "test-ns")

	diskSpecsArg := findArgWithPrefix(args, "--disk-specs=")
	if diskSpecsArg == "" {
		t.Fatal("missing --disk-specs argument")
	}
	diskSpecsJSON := strings.TrimPrefix(diskSpecsArg, "--disk-specs=")

	var diskSpecs []struct {
		LunID      int    `json:"lunId"`
		VolumePath string `json:"volumePath"`
	}
	if err := json.Unmarshal([]byte(diskSpecsJSON), &diskSpecs); err != nil {
		t.Fatalf("failed to parse disk-specs JSON: %v", err)
	}
	if len(diskSpecs) != 1 {
		t.Fatalf("expected 1 disk spec, got %d", len(diskSpecs))
	}
	if diskSpecs[0].LunID != 0 {
		t.Errorf("expected LUN ID 0, got %d", diskSpecs[0].LunID)
	}
	if diskSpecs[0].VolumePath != v1beta1.HyperVPopulatorBlockDevicePath {
		t.Errorf("expected volume path %q (rawBlock HyperV), got %q", v1beta1.HyperVPopulatorBlockDevicePath, diskSpecs[0].VolumePath)
	}
}

func TestGetHyperVPopulatorPodArgs_FilesystemMode(t *testing.T) {
	cr := map[string]interface{}{
		"apiVersion": "forklift.konveyor.io/v1beta1",
		"kind":       "HyperVVolumePopulator",
		"metadata": map[string]interface{}{
			"name":      "test-populator",
			"namespace": "test-ns",
		},
		"spec": map[string]interface{}{
			"secretName":   "my-secret",
			"vmId":         "vm-123",
			"vmName":       "TestVM",
			"diskIndex":    int64(0),
			"diskPath":     `C:\VMs\TestVM\disk0.vhdx`,
			"targetIQN":    "iqn.2026-03.io.forklift:vm-123",
			"targetPortal": "10.0.0.100:3260",
			"lunId":        int64(1),
			"initiatorIQN": "iqn.2026-03.io.forklift:copy-migration-uid",
		},
	}

	u := &unstructured.Unstructured{Object: cr}
	pvc := corev1.PersistentVolumeClaim{}

	args, err := getHyperVPopulatorPodArgs(false, u, pvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertArgPresent(t, args, "--portal=", "10.0.0.100:3260")
	assertArgPresent(t, args, "--target-iqn=", "iqn.2026-03.io.forklift:vm-123")
	assertArgPresent(t, args, "--initiator-iqn=", "iqn.2026-03.io.forklift:copy-migration-uid")
	assertArgPresent(t, args, "--cr-name=", "test-populator")
	assertArgPresent(t, args, "--cr-namespace=", "test-ns")

	diskSpecsArg := findArgWithPrefix(args, "--disk-specs=")
	if diskSpecsArg == "" {
		t.Fatal("missing --disk-specs argument")
	}
	diskSpecsJSON := strings.TrimPrefix(diskSpecsArg, "--disk-specs=")

	var diskSpecs []struct {
		LunID      int    `json:"lunId"`
		VolumePath string `json:"volumePath"`
	}
	if err := json.Unmarshal([]byte(diskSpecsJSON), &diskSpecs); err != nil {
		t.Fatalf("failed to parse disk-specs JSON: %v", err)
	}
	if len(diskSpecs) != 1 {
		t.Fatalf("expected 1 disk spec, got %d", len(diskSpecs))
	}
	if diskSpecs[0].LunID != 1 {
		t.Errorf("expected LUN ID 1, got %d", diskSpecs[0].LunID)
	}
	expectedPath := mountPath + "disk.img"
	if diskSpecs[0].VolumePath != expectedPath {
		t.Errorf("expected volume path %q (filesystem), got %q", expectedPath, diskSpecs[0].VolumePath)
	}
}

func assertArgPresent(t *testing.T, args []string, prefix, expectedValue string) {
	t.Helper()
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			got := strings.TrimPrefix(arg, prefix)
			if got != expectedValue {
				t.Errorf("arg %s: expected %q, got %q", prefix, expectedValue, got)
			}
			return
		}
	}
	t.Errorf("missing argument with prefix %q", prefix)
}

func findArgWithPrefix(args []string, prefix string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return arg
		}
	}
	return ""
}
