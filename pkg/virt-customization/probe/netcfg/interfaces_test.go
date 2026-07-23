package netcfg

import "testing"

func TestInterfaces(t *testing.T) {
	t.Parallel()
	section := "auto lo\niface lo inet loopback\n\nauto eth0\niface eth0 inet static\naddress 192.168.1.200\nnetmask 255.255.255.0\n\nauto eth1\niface eth1 inet dhcp\n"
	interfaces, err := Interfaces(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.200") {
		t.Error("expected IPv4 192.168.1.200")
	}
	if interfaces[0].Source != "interfaces" {
		t.Errorf("expected source interfaces, got %s", interfaces[0].Source)
	}
	if interfaces[0].DHCP {
		t.Error("expected eth0 DHCP to be false")
	}
	if !interfaces[1].DHCP {
		t.Error("expected eth1 DHCP to be true")
	}
}

func TestInterfaces_IPv6(t *testing.T) {
	t.Parallel()
	section := "auto eth0\niface eth0 inet static\naddress 192.168.1.200\n\niface eth0 inet6 static\naddress fd00::1\n"
	interfaces, err := Interfaces(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interface stanzas, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.200") {
		t.Error("expected IPv4 192.168.1.200")
	}
	if len(interfaces[0].IPv6) != 0 {
		t.Errorf("expected no IPv6 on inet stanza, got %v", interfaces[0].IPv6)
	}
	if !containsStr(interfaces[1].IPv6, "fd00::1") {
		t.Error("expected IPv6 fd00::1")
	}
	if len(interfaces[1].IPv4) != 0 {
		t.Errorf("expected no IPv4 on inet6 stanza, got %v", interfaces[1].IPv4)
	}
}
