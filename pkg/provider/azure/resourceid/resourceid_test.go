package resourceid

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    Parsed
		wantErr bool
	}{
		{
			name: "VM",
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/my-vm",
			want: Parsed{
				Subscription:  "sub1",
				ResourceGroup: "rg1",
				Provider:      "Microsoft.Compute",
				ResourceType:  "virtualMachines",
				Name:          "my-vm",
			},
		},
		{
			name: "managed disk",
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/disks/os-disk-0",
			want: Parsed{
				Subscription:  "sub1",
				ResourceGroup: "rg1",
				Provider:      "Microsoft.Compute",
				ResourceType:  "disks",
				Name:          "os-disk-0",
			},
		},
		{
			name: "subnet (nested resource)",
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/default",
			want: Parsed{
				Subscription:  "sub1",
				ResourceGroup: "rg1",
				Provider:      "Microsoft.Network",
				ResourceType:  "virtualNetworks",
				Name:          "vnet1",
				SubType:       "subnets",
				SubName:       "default",
			},
		},
		{
			name:    "empty string",
			id:      "",
			wantErr: true,
		},
		{
			name:    "missing providers segment",
			id:      "/subscriptions/sub1/resourceGroups/rg1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuild(t *testing.T) {
	got := Build("sub1", "rg1", ComputeProvider, VMType, "my-vm")
	want := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/my-vm"
	if got != want {
		t.Errorf("Build() = %q, want %q", got, want)
	}
}

func TestBuildNested(t *testing.T) {
	got := BuildNested("sub1", "rg1", NetworkProvider, VNetType, "vnet1", SubnetType, "default")
	want := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/default"
	if got != want {
		t.Errorf("BuildNested() = %q, want %q", got, want)
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/my-vm",
			want: "my-vm",
		},
		{
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/default",
			want: "default",
		},
		{id: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := Name(tt.id); got != tt.want {
				t.Errorf("Name(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestQualifiedID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{
			name: "VM",
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/my-vm",
			want: "rg1--my-vm",
		},
		{
			name: "nested subnet",
			id:   "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/virtualNetworks/vnet1/subnets/default",
			want: "rg1--default",
		},
		{
			name:    "invalid ARM ID",
			id:      "not-an-arm-id",
			wantErr: true,
		},
		{
			name:    "separator in resource group",
			id:      "/subscriptions/sub1/resourceGroups/my--rg/providers/Microsoft.Compute/virtualMachines/vm1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QualifiedID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("QualifiedID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("QualifiedID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQualifiedIDLengthLimit(t *testing.T) {
	longRG := strings.Repeat("a", 90)
	longName := strings.Repeat("b", 200)
	armID := "/subscriptions/sub1/resourceGroups/" + longRG +
		"/providers/Microsoft.Compute/virtualMachines/" + longName
	_, err := QualifiedID(armID)
	if err == nil {
		t.Error("expected error for qualified ID exceeding length limit")
	}
}

func TestSplitQualifiedID(t *testing.T) {
	tests := []struct {
		input  string
		wantRG string
		wantN  string
	}{
		{"rg1--my-vm", "rg1", "my-vm"},
		{"rg1--name--with--dashes", "rg1", "name--with--dashes"},
		{"no-separator", "", "no-separator"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rg, name := SplitQualifiedID(tt.input)
			if rg != tt.wantRG || name != tt.wantN {
				t.Errorf("SplitQualifiedID(%q) = (%q, %q), want (%q, %q)", tt.input, rg, name, tt.wantRG, tt.wantN)
			}
		})
	}
}

func TestParseRoundTrip(t *testing.T) {
	original := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/my-vm"
	p, err := Parse(original)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt := Build(p.Subscription, p.ResourceGroup, p.Provider, p.ResourceType, p.Name)
	if rebuilt != original {
		t.Errorf("round-trip failed: got %q, want %q", rebuilt, original)
	}
}
