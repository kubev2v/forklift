package powershell

import (
	"strings"
	"testing"
)

func TestGetCheckpointDiskPathsWalksLegacyAVHD(t *testing.T) {
	if !strings.Contains(GetCheckpointDiskPaths, "*.avhdx") {
		t.Fatal("expected Root walk to include *.avhdx")
	}
	if !strings.Contains(GetCheckpointDiskPaths, "*.avhd") {
		t.Fatal("expected Root walk to include legacy *.avhd")
	}
	if !strings.Contains(GetCheckpointDiskPaths, "Get-VHD failed for active disk") {
		t.Fatal("expected active-disk Get-VHD failure to be surfaced")
	}
	if !strings.Contains(GetCheckpointDiskPaths, "Get-VHD failed for root disk") {
		t.Fatal("expected root-disk Get-VHD failure to be surfaced")
	}
	if !strings.Contains(GetCheckpointDiskPaths, "Get-VHD failed for chain disk") {
		t.Fatal("expected chain-disk Get-VHD failure to be surfaced")
	}
}
