package hyperv

import (
	"strings"
	"testing"
)

func TestIscsiTargetName(t *testing.T) {
	tests := []struct {
		name   string
		vmID   string
		expect string
	}{
		{
			name:   "simple UUID",
			vmID:   "abc-123-def-456",
			expect: "forklift-abc123def456",
		},
		{
			name:   "no hyphens",
			vmID:   "abc123",
			expect: "forklift-abc123",
		},
		{
			name:   "ID portion truncated to 40 chars",
			vmID:   "aaaaaaaa-bbbbbbbb-cccccccc-dddddddd-eeeeeeee-ffffffff",
			expect: "forklift-aaaaaaaabbbbbbbbccccccccddddddddeeeeeeee",
		},
		{
			name:   "empty ID",
			vmID:   "",
			expect: "forklift-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := iscsiTargetName(tt.vmID)
			if got != tt.expect {
				t.Errorf("iscsiTargetName(%q) = %q, want %q", tt.vmID, got, tt.expect)
			}
		})
	}
}

func TestIscsiTargetName_Deterministic(t *testing.T) {
	a := iscsiTargetName("test-vm-id")
	b := iscsiTargetName("test-vm-id")
	if a != b {
		t.Errorf("iscsiTargetName should be deterministic: %q != %q", a, b)
	}
}

func TestIscsiTargetName_Prefix(t *testing.T) {
	got := iscsiTargetName("any-id")
	if !strings.HasPrefix(got, "forklift-") {
		t.Errorf("expected forklift- prefix, got %q", got)
	}
}
