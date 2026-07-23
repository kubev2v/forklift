package netcfg

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

func findIface(interfaces []api.InterfaceInfo, name string) *api.InterfaceInfo {
	for i := range interfaces {
		if interfaces[i].Name == name {
			return &interfaces[i]
		}
	}
	return nil
}

func TestNetplan(t *testing.T) {
	t.Parallel()
	section := `network:
  ethernets:
    eth0:
      addresses:
        - 192.168.1.50/24
        - 10.0.0.1/8
    eth1:
      dhcp4: true
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(result.Interfaces))
	}
	eth0 := findIface(result.Interfaces, "eth0")
	if eth0 == nil {
		t.Fatal("expected to find eth0")
		return
	}
	if !containsStr(eth0.IPv4, "192.168.1.50") {
		t.Error("expected IPv4 192.168.1.50")
	}
	if !containsStr(eth0.IPv4, "10.0.0.1") {
		t.Error("expected IPv4 10.0.0.1")
	}
	if eth0.DHCP {
		t.Error("expected eth0 DHCP to be false")
	}
	eth1 := findIface(result.Interfaces, "eth1")
	if eth1 == nil {
		t.Fatal("expected to find eth1")
		return
	}
	if !eth1.DHCP {
		t.Error("expected eth1 DHCP to be true")
	}
}

func TestNetplan_4SpaceIndent(t *testing.T) {
	t.Parallel()
	section := `network:
    ethernets:
        eth0:
            addresses:
                - 192.168.1.50/24
        eth1:
            dhcp4: true
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(result.Interfaces))
	}
	eth0 := findIface(result.Interfaces, "eth0")
	if eth0 == nil {
		t.Fatal("expected to find eth0")
		return
	}
	if !containsStr(eth0.IPv4, "192.168.1.50") {
		t.Error("expected IPv4 192.168.1.50")
	}
	if findIface(result.Interfaces, "eth1") == nil {
		t.Error("expected to find eth1")
	}
}

func TestNetplan_MACAddress(t *testing.T) {
	t.Parallel()
	section := `network:
  ethernets:
    eth0:
      match:
        macaddress: "00:11:22:33:44:55"
      addresses:
        - 192.168.1.50/24
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(result.Interfaces))
	}
	if result.Interfaces[0].MAC != "00:11:22:33:44:55" {
		t.Errorf("expected MAC 00:11:22:33:44:55, got %s", result.Interfaces[0].MAC)
	}
}

func TestNetplan_MultipleDocuments(t *testing.T) {
	t.Parallel()
	section := `network:
  ethernets:
    eth0:
      addresses:
        - 192.168.1.50/24
---
network:
  ethernets:
    eth1:
      addresses:
        - 10.0.0.1/8
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(result.Interfaces))
	}
	if findIface(result.Interfaces, "eth0") == nil {
		t.Error("expected to find eth0")
	}
	if findIface(result.Interfaces, "eth1") == nil {
		t.Error("expected to find eth1")
	}
}

func TestNetplan_Renderer(t *testing.T) {
	t.Parallel()
	section := `network:
  renderer: networkd
  ethernets:
    eth0:
      dhcp4: true
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Renderer != "networkd" {
		t.Errorf("expected renderer networkd, got %s", result.Renderer)
	}
	if len(result.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(result.Interfaces))
	}
	if !result.Interfaces[0].DHCP {
		t.Error("expected DHCP to be true")
	}
}

func TestNetplan_IPv6(t *testing.T) {
	t.Parallel()
	section := `network:
  ethernets:
    eth0:
      addresses:
        - 192.168.1.50/24
        - "fd00::1/64"
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(result.Interfaces))
	}
	eth0 := findIface(result.Interfaces, "eth0")
	if eth0 == nil {
		t.Fatal("expected to find eth0")
		return
	}
	if !containsStr(eth0.IPv4, "192.168.1.50") {
		t.Error("expected IPv4 192.168.1.50")
	}
	if !containsStr(eth0.IPv6, "fd00::1") {
		t.Error("expected IPv6 fd00::1")
	}
}

func TestNetplan_EmptyEthernets(t *testing.T) {
	t.Parallel()
	section := `network:
  ethernets: {}
`
	result, err := Netplan(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty ethernets, got %d", len(result.Interfaces))
	}
}

func TestNetplan_EmptyInput(t *testing.T) {
	t.Parallel()
	result, err := Netplan("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(result.Interfaces))
	}
}

func TestNetplan_InvalidYAML(t *testing.T) {
	t.Parallel()
	section := "not: valid: yaml: [[[["
	result, err := Netplan(section)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
	if len(result.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces for invalid YAML, got %d", len(result.Interfaces))
	}
}
