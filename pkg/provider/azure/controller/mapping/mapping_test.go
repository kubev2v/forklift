package mapping

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestFindStorageClass(t *testing.T) {
	storageMap := &api.StorageMap{
		Spec: api.StorageMapSpec{
			Map: []api.StoragePair{
				{
					Source:      ref.Ref{Name: "Premium_LRS"},
					Destination: api.DestinationStorage{StorageClass: "managed-premium"},
				},
				{
					Source:      ref.Ref{Name: "Standard_LRS"},
					Destination: api.DestinationStorage{StorageClass: "managed-standard"},
				},
			},
		},
	}

	tests := []struct {
		name    string
		diskSKU string
		want    string
	}{
		{"Premium_LRS maps to managed-premium", "Premium_LRS", "managed-premium"},
		{"Standard_LRS maps to managed-standard", "Standard_LRS", "managed-standard"},
		{"unknown SKU returns empty", "UltraSSD_LRS", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindStorageClass(storageMap, tt.diskSKU)
			if got != tt.want {
				t.Errorf("FindStorageClass(%q) = %q, want %q", tt.diskSKU, got, tt.want)
			}
		})
	}
}

func TestFindStorageClass_NilMap(t *testing.T) {
	got := FindStorageClass(nil, "Premium_LRS")
	if got != "" {
		t.Errorf("FindStorageClass(nil, ...) = %q, want empty", got)
	}
}

func TestHasStorageMapping(t *testing.T) {
	storageMap := &api.StorageMap{
		Spec: api.StorageMapSpec{
			Map: []api.StoragePair{
				{
					Source:      ref.Ref{Name: "Premium_LRS"},
					Destination: api.DestinationStorage{StorageClass: "sc"},
				},
			},
		},
	}

	if !HasStorageMapping(storageMap, "Premium_LRS") {
		t.Error("expected HasStorageMapping=true for Premium_LRS")
	}
	if HasStorageMapping(storageMap, "Standard_LRS") {
		t.Error("expected HasStorageMapping=false for Standard_LRS")
	}
}

func TestFindNetworkPair_ByID(t *testing.T) {
	networkMap := &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source: api.NetworkSourceRef{
						Ref: ref.Ref{
							ID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet1",
							Name: "subnet1",
						},
					},
					Destination: api.DestinationNetwork{
						Type: "pod",
					},
				},
			},
		},
	}

	pair := FindNetworkPair(networkMap, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet1")
	if pair == nil {
		t.Fatal("expected to find network pair by ID")
	}
	if pair.Destination.Type != "pod" {
		t.Errorf("destination type = %q, want pod", pair.Destination.Type)
	}
}

func TestFindNetworkPair_ByName(t *testing.T) {
	networkMap := &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source: api.NetworkSourceRef{
						Ref: ref.Ref{Name: "subnet1"},
					},
					Destination: api.DestinationNetwork{
						Type: "multus",
						Name: "my-nad",
					},
				},
			},
		},
	}

	pair := FindNetworkPair(networkMap, "subnet1")
	if pair == nil {
		t.Fatal("expected to find network pair by name")
	}
	if pair.Destination.Name != "my-nad" {
		t.Errorf("destination name = %q, want my-nad", pair.Destination.Name)
	}
}

func TestFindNetworkPair_NotFound(t *testing.T) {
	networkMap := &api.NetworkMap{
		Spec: api.NetworkMapSpec{
			Map: []api.NetworkPair{
				{
					Source: api.NetworkSourceRef{
						Ref: ref.Ref{Name: "subnet1"},
					},
				},
			},
		},
	}

	pair := FindNetworkPair(networkMap, "subnet-other")
	if pair != nil {
		t.Error("expected nil for non-matching subnet")
	}
}

func TestFindNetworkPair_NilMap(t *testing.T) {
	pair := FindNetworkPair(nil, "subnet1")
	if pair != nil {
		t.Error("expected nil for nil network map")
	}
}
