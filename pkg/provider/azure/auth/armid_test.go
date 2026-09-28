package auth

import "testing"

func TestParseResourceGroup(t *testing.T) {
	tests := []struct {
		name    string
		armID   string
		want    string
		wantErr bool
	}{
		{
			name:  "virtual machine",
			armID: "/subscriptions/sub-1/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/vm-1",
			want:  "my-rg",
		},
		{
			name:  "subnet",
			armID: "/subscriptions/sub-1/resourceGroups/net-rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet-1",
			want:  "net-rg",
		},
		{
			name:    "missing resource group",
			armID:   "/subscriptions/sub-1/providers/Microsoft.Compute/virtualMachines/vm-1",
			wantErr: true,
		},
		{
			name:    "empty",
			armID:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourceGroup(tt.armID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ParseResourceGroup() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseResourceName(t *testing.T) {
	tests := []struct {
		armID string
		want  string
	}{
		{
			armID: "/subscriptions/sub-1/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachines/vm-1",
			want:  "vm-1",
		},
		{
			armID: "/subscriptions/sub-1/resourceGroups/net-rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet-1",
			want:  "subnet-1",
		},
		{
			armID: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		got := ParseResourceName(tt.armID)
		if got != tt.want {
			t.Errorf("ParseResourceName(%q) = %q, want %q", tt.armID, got, tt.want)
		}
	}
}

func TestIsARMResourceID(t *testing.T) {
	if !IsARMResourceID("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm") {
		t.Error("expected ARM resource ID to be recognized")
	}
	if IsARMResourceID("vm-name") {
		t.Error("expected plain name not to be recognized as ARM resource ID")
	}
}
