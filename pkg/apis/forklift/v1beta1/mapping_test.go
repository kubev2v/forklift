package v1beta1

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestFindStorageMergesSplitOffloadPlugins(t *testing.T) {
	sm := &StorageMap{
		Spec: StorageMapSpec{
			Map: []StoragePair{
				{
					Source:      ref.Ref{ID: "datastore-1"},
					Destination: DestinationStorage{StorageClass: "sc-a"},
					OffloadPlugin: &OffloadPlugin{
						CsiVolumeImport: &CsiVolumeImport{
							SecretRef:            "csi-secret",
							StorageVendorProduct: StorageVendorProductPrimera3Par,
						},
					},
				},
				{
					Source:      ref.Ref{ID: "datastore-1"},
					Destination: DestinationStorage{StorageClass: "sc-a"},
					OffloadPlugin: &OffloadPlugin{
						VSphereXcopyPluginConfig: &VSphereXcopyPluginConfig{
							SecretRef:            "xcopy-secret",
							StorageVendorProduct: StorageVendorProductPrimera3Par,
						},
					},
				},
			},
		},
	}

	pair, found := sm.FindStorage("datastore-1")
	if !found {
		t.Fatal("expected datastore mapping")
	}
	if pair.OffloadPlugin == nil || pair.OffloadPlugin.CsiVolumeImport == nil {
		t.Fatal("expected CSI import config")
	}
	if pair.OffloadPlugin.VSphereXcopyPluginConfig == nil {
		t.Fatal("expected xcopy config")
	}
}
