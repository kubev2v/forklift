package populator

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"github.com/kubev2v/forklift/pkg/storage/resolver"
)

// rdmIncapableStorageApi models ONTAP, PowerStore, PowerMax, Infinibox, Vantara, and
// PowerFlex: none of them implement RDMCapable today.
type rdmIncapableStorageApi struct {
	VMDKCapable
}

// rdmCapableStorageApi models 3PAR, Pure, and FlashSystem, which do implement RDMCapable.
type rdmCapableStorageApi struct {
	VMDKCapable
}

func (r *rdmCapableStorageApi) RDMCopy(vmware.Client, string, string, PersistentVolume, chan<- uint64) error {
	return nil
}

func TestCanUse_RDM(t *testing.T) {
	settings.RDMDisabled = false

	tests := []struct {
		name       string
		storageApi StorageApi
		want       bool
	}{
		{"RDM-incapable destination falls through to VMDK/Xcopy path", &rdmIncapableStorageApi{}, false},
		{"RDM-capable destination uses the RDM path", &rdmCapableStorageApi{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canUse(tt.storageApi, resolver.DiskTypeRDM); got != tt.want {
				t.Errorf("canUse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanUse_RDMDisabledOverridesCapability(t *testing.T) {
	settings.RDMDisabled = true
	defer func() { settings.RDMDisabled = false }()

	if canUse(&rdmCapableStorageApi{}, resolver.DiskTypeRDM) {
		t.Error("canUse() = true, want false: DISABLE_RDM_METHOD must force the VMDK/Xcopy path even for an RDM-capable destination")
	}
}
