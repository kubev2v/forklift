package nutanix

import (
	"encoding/json"
	"testing"

	libclient "github.com/kubev2v/forklift/pkg/lib/client/nutanix"
)

func TestNutanixBool(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"native true", true, true},
		{"native false", false, false},
		{"enabled string", "ENABLED", true},
		{"disabled string", "DISABLED", false},
		{"true string", "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := libclient.NutanixBool(tt.value); got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestListAllV3DecodeVMWithStringFlashMode(t *testing.T) {
	payload := []byte(`{
		"metadata": {"total_matches": 1},
		"entities": [{
			"metadata": {"uuid": "vm-1"},
			"spec": {
				"name": "test-vm",
				"resources": {
					"disk_list": [{
						"uuid": "disk-1",
						"device_properties": {
							"device_type": "DISK",
							"disk_address": {"device_index": 0, "adapter_type": "SCSI"}
						},
						"storage_config": {
							"flash_mode": "DISABLED"
						}
					}]
				}
			},
			"status": {"resources": {}}
		}]
	}`)

	var response libclient.V3ListResponse[vmEntity]
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("failed to decode VM list: %v", err)
	}

	disk := applyDisk(&response.Entities[0].Spec.Resources.DiskList[0])
	if disk.FlashMode {
		t.Fatal("expected flash_mode DISABLED to map to false")
	}
}
