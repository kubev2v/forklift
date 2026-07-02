package netcfg

import "testing"

func TestNM(t *testing.T) {
	t.Parallel()
	section := "[connection]\nid=Wired connection 1\ntype=ethernet\ninterface-name=ens192\n\n[ipv4]\naddress1=192.168.1.100/24,192.168.1.1\nmethod=manual\n\n[ethernet]\nmac-address=00:50:56:a1:b2:c3\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].Name != "ens192" {
		t.Errorf("expected ens192, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.100") {
		t.Error("expected IPv4 192.168.1.100")
	}
	if interfaces[0].MAC != "00:50:56:a1:b2:c3" {
		t.Errorf("expected MAC 00:50:56:a1:b2:c3, got %s", interfaces[0].MAC)
	}
	if interfaces[0].DHCP {
		t.Error("expected DHCP to be false for method=manual")
	}
}

func TestNM_DHCP(t *testing.T) {
	t.Parallel()
	section := "[connection]\nid=Wired connection 1\ntype=ethernet\ninterface-name=ens192\n\n[ipv4]\nmethod=auto\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !interfaces[0].DHCP {
		t.Error("expected DHCP to be true for method=auto")
	}
}

func TestNM_MultiProfile(t *testing.T) {
	t.Parallel()
	section := "[connection]\nid=Wired connection 1\ntype=ethernet\ninterface-name=ens192\n\n[ipv4]\naddress1=192.168.1.100/24,192.168.1.1\nmethod=manual\n\n[ethernet]\nmac-address=00:50:56:a1:b2:c3\n[connection]\nid=Wired connection 2\ntype=ethernet\ninterface-name=ens224\n\n[ipv4]\naddress1=10.0.0.50/16,10.0.0.1\nmethod=manual\n\n[ethernet]\nmac-address=00:50:56:d4:e5:f6\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if interfaces[0].Name != "ens192" {
		t.Errorf("expected ens192, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.100") {
		t.Error("expected IPv4 192.168.1.100 on first interface")
	}
	if interfaces[0].MAC != "00:50:56:a1:b2:c3" {
		t.Errorf("expected MAC 00:50:56:a1:b2:c3, got %s", interfaces[0].MAC)
	}
	if interfaces[1].Name != "ens224" {
		t.Errorf("expected ens224, got %s", interfaces[1].Name)
	}
	if !containsStr(interfaces[1].IPv4, "10.0.0.50") {
		t.Error("expected IPv4 10.0.0.50 on second interface")
	}
	if interfaces[1].MAC != "00:50:56:d4:e5:f6" {
		t.Errorf("expected MAC 00:50:56:d4:e5:f6, got %s", interfaces[1].MAC)
	}
}

func TestNM_IPv6(t *testing.T) {
	t.Parallel()
	section := "[connection]\nid=Wired connection 1\ntype=ethernet\ninterface-name=ens192\n\n[ipv4]\naddress1=192.168.1.100/24,192.168.1.1\nmethod=manual\n\n[ipv6]\naddress1=fd00::1/64\nmethod=manual\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.100") {
		t.Error("expected IPv4 192.168.1.100")
	}
	if !containsStr(interfaces[0].IPv6, "fd00::1") {
		t.Error("expected IPv6 fd00::1")
	}
	if len(interfaces[0].IPv4) != 1 {
		t.Errorf("expected 1 IPv4, got %d", len(interfaces[0].IPv4))
	}
	if len(interfaces[0].IPv6) != 1 {
		t.Errorf("expected 1 IPv6, got %d", len(interfaces[0].IPv6))
	}
}

func TestNM_NoConnectionHeader(t *testing.T) {
	t.Parallel()
	section := "interface-name=eth0\n\n[ipv4]\naddress1=10.0.0.1/24,10.0.0.254\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
}

func TestNM_NonIPSectionsIgnoreAddresses(t *testing.T) {
	t.Parallel()
	section := "[connection]\nid=Test\ntype=ethernet\ninterface-name=ens192\n\n[proxy]\nmethod=auto\naddress1=1.2.3.4/24,1.2.3.1\n\n[ipv4]\naddress1=192.168.1.100/24,192.168.1.1\nmethod=manual\n"
	interfaces, err := NM(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].Name != "ens192" {
		t.Errorf("expected ens192, got %s", interfaces[0].Name)
	}
	if len(interfaces[0].IPv4) != 1 {
		t.Errorf("expected 1 IPv4 address, got %d: %v", len(interfaces[0].IPv4), interfaces[0].IPv4)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.100") {
		t.Error("expected IPv4 192.168.1.100")
	}
	if interfaces[0].DHCP {
		t.Error("expected DHCP false: method=auto in [proxy] should not set DHCP")
	}
}

func TestNM_EmptyInput(t *testing.T) {
	t.Parallel()
	interfaces, err := NM("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(interfaces))
	}
}
