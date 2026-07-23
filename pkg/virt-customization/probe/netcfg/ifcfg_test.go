package netcfg

import "testing"

func TestIfcfg(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nIPADDR=192.168.1.10\nHWADDR=00:11:22:33:44:55\n\nDEVICE=eth1\nIPADDR=10.0.0.5\nHWADDR=aa:bb:cc:dd:ee:ff\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.10") {
		t.Error("expected IPv4 192.168.1.10")
	}
	if interfaces[0].MAC != "00:11:22:33:44:55" {
		t.Errorf("expected MAC 00:11:22:33:44:55, got %s", interfaces[0].MAC)
	}
	if interfaces[0].Source != "ifcfg" {
		t.Errorf("expected source ifcfg, got %s", interfaces[0].Source)
	}
	if interfaces[1].Name != "eth1" {
		t.Errorf("expected eth1, got %s", interfaces[1].Name)
	}
}

func TestIfcfg_DHCP(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nBOOTPROTO=dhcp\nHWADDR=00:11:22:33:44:55\n\nDEVICE=eth1\nBOOTPROTO=static\nIPADDR=10.0.0.5\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if !interfaces[0].DHCP {
		t.Error("expected eth0 DHCP to be true")
	}
	if interfaces[1].DHCP {
		t.Error("expected eth1 DHCP to be false")
	}
}

func TestIfcfg_NumberedIPADDR(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nIPADDR0=192.168.1.10\nIPADDR1=10.0.0.1\nHWADDR=00:11:22:33:44:55\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.10") {
		t.Error("expected IPv4 192.168.1.10")
	}
	if !containsStr(interfaces[0].IPv4, "10.0.0.1") {
		t.Error("expected IPv4 10.0.0.1")
	}
}

func TestIfcfg_IPv6(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nIPADDR=192.168.1.10\nIPV6ADDR=fd00::1/64\nIPV6ADDR_SECONDARIES=\"fd00::2/64 fd00::3/64\"\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.10") {
		t.Error("expected IPv4 192.168.1.10")
	}
	if !containsStr(interfaces[0].IPv6, "fd00::1") {
		t.Error("expected IPv6 fd00::1")
	}
	if !containsStr(interfaces[0].IPv6, "fd00::2") {
		t.Error("expected IPv6 fd00::2")
	}
	if !containsStr(interfaces[0].IPv6, "fd00::3") {
		t.Error("expected IPv6 fd00::3")
	}
	if len(interfaces[0].IPv6) != 3 {
		t.Errorf("expected 3 IPv6 addresses, got %d", len(interfaces[0].IPv6))
	}
}

func TestIfcfg_EmptyIPADDR(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nIPADDR=\nHWADDR=00:11:22:33:44:55\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if len(interfaces[0].IPv4) != 0 {
		t.Errorf("expected no IPv4 for empty IPADDR, got %v", interfaces[0].IPv4)
	}
}

func TestIfcfg_ConcatenatedRecords(t *testing.T) {
	t.Parallel()
	section := "DEVICE=eth0\nIPADDR=192.168.1.10\nHWADDR=00:11:22:33:44:55\nDEVICE=eth1\nIPADDR=10.0.0.5\nHWADDR=aa:bb:cc:dd:ee:ff\n"
	interfaces, err := Ifcfg(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.10") {
		t.Error("expected IPv4 192.168.1.10 on eth0")
	}
	if interfaces[0].MAC != "00:11:22:33:44:55" {
		t.Errorf("expected MAC 00:11:22:33:44:55 on eth0, got %s", interfaces[0].MAC)
	}
	if interfaces[1].Name != "eth1" {
		t.Errorf("expected eth1, got %s", interfaces[1].Name)
	}
	if !containsStr(interfaces[1].IPv4, "10.0.0.5") {
		t.Error("expected IPv4 10.0.0.5 on eth1")
	}
	if interfaces[1].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected MAC aa:bb:cc:dd:ee:ff on eth1, got %s", interfaces[1].MAC)
	}
}

func TestIfcfg_EmptyInput(t *testing.T) {
	t.Parallel()
	interfaces, err := Ifcfg("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(interfaces))
	}
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
