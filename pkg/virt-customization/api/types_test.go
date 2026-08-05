package api

import "testing"

func TestInterfaceForIP(t *testing.T) {
	t.Parallel()
	guest := &GuestInfo{
		Interfaces: []InterfaceInfo{
			{Name: "eth0", IPv4: []string{"10.0.0.1"}, IPv6: []string{"fd00::1"}},
			{Name: "eth1", IPv4: []string{"192.168.1.5"}},
		},
	}

	tests := []struct {
		name string
		ip   string
		want string
	}{
		{"ipv4 hit", "10.0.0.1", "eth0"},
		{"ipv6 hit", "fd00::1", "eth0"},
		{"second iface", "192.168.1.5", "eth1"},
		{"miss", "172.16.0.1", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := guest.InterfaceForIP(tc.ip); got != tc.want {
				t.Errorf("InterfaceForIP(%q) = %q, want %q", tc.ip, got, tc.want)
			}
		})
	}
}

func TestInterfaceForIP_Empty(t *testing.T) {
	t.Parallel()
	guest := &GuestInfo{}
	if got := guest.InterfaceForIP("10.0.0.1"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestHasIPs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info InterfaceInfo
		want bool
	}{
		{"ipv4 only", InterfaceInfo{IPv4: []string{"10.0.0.1"}}, true},
		{"ipv6 only", InterfaceInfo{IPv6: []string{"fd00::1"}}, true},
		{"both", InterfaceInfo{IPv4: []string{"10.0.0.1"}, IPv6: []string{"fd00::1"}}, true},
		{"empty", InterfaceInfo{}, false},
		{"nil slices", InterfaceInfo{Name: "eth0"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.info.HasIPs(); got != tc.want {
				t.Errorf("HasIPs() = %v, want %v", got, tc.want)
			}
		})
	}
}
