package netcfg

import "testing"

func TestWicked(t *testing.T) {
	files := "duid.xml\nlease-eth0-dhcp-ipv4.xml\nlease-eth1-dhcp-ipv4.xml\n"
	xml := `<lease>
    <family>ipv4</family>
    <type>dhcp</type>
    <state>granted</state>
    <ipv4:dhcp>
        <address>192.168.122.82</address>
    </ipv4:dhcp>
</lease>
<lease>
    <family>ipv4</family>
    <type>dhcp</type>
    <state>granted</state>
    <ipv4:dhcp>
        <address>192.168.122.83</address>
    </ipv4:dhcp>
</lease>
`
	interfaces, err := Wicked(files, xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.122.82") {
		t.Error("expected IPv4 192.168.122.82")
	}
	if len(interfaces[0].IPv6) != 0 {
		t.Errorf("expected no IPv6, got %v", interfaces[0].IPv6)
	}
	if interfaces[0].Source != "wicked" {
		t.Errorf("expected source wicked, got %s", interfaces[0].Source)
	}
	if interfaces[1].Name != "eth1" {
		t.Errorf("expected eth1, got %s", interfaces[1].Name)
	}
	if !containsStr(interfaces[1].IPv4, "192.168.122.83") {
		t.Error("expected IPv4 192.168.122.83")
	}
}

func TestWicked_IPv6(t *testing.T) {
	files := "lease-eth0-dhcp-ipv6.xml\n"
	xml := `<lease>
    <family>ipv6</family>
    <type>dhcp</type>
    <state>granted</state>
    <ipv6:dhcp>
        <address>fd00::42</address>
    </ipv6:dhcp>
</lease>
`
	interfaces, err := Wicked(files, xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv6, "fd00::42") {
		t.Error("expected IPv6 fd00::42")
	}
	if len(interfaces[0].IPv4) != 0 {
		t.Errorf("expected no IPv4, got %v", interfaces[0].IPv4)
	}
}

func TestWicked_NoLeaseFiles(t *testing.T) {
	files := "duid.xml\n"
	xml := ""
	interfaces, err := Wicked(files, xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces, got %d", len(interfaces))
	}
}

func TestParseLeaseFilenames(t *testing.T) {
	section := "duid.xml\nlease-eth0-dhcp-ipv4.xml\nlease-ens192-dhcp-ipv6.xml\n"
	leases := parseLeaseFilenames(section)
	if len(leases) != 2 {
		t.Fatalf("expected 2 leases, got %d", len(leases))
	}
	if leases[0].name != "eth0" {
		t.Errorf("expected eth0, got %s", leases[0].name)
	}
	if leases[0].family != "ipv4" {
		t.Errorf("expected ipv4, got %s", leases[0].family)
	}
	if leases[1].name != "ens192" {
		t.Errorf("expected ens192, got %s", leases[1].name)
	}
	if leases[1].family != "ipv6" {
		t.Errorf("expected ipv6, got %s", leases[1].family)
	}
}

func TestWicked_MoreFilesThanXML(t *testing.T) {
	files := "lease-eth0-dhcp-ipv4.xml\nlease-eth1-dhcp-ipv4.xml\nlease-eth2-dhcp-ipv4.xml\n"
	xml := `<lease><ipv4:dhcp><address>192.168.1.1</address></ipv4:dhcp></lease>`
	interfaces, err := Wicked(files, xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 3 {
		t.Fatalf("expected 3 interfaces, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.1.1") {
		t.Error("expected first interface to have IPv4 192.168.1.1")
	}
	if len(interfaces[1].IPv4) != 0 {
		t.Errorf("expected second interface to have no IPs, got %v", interfaces[1].IPv4)
	}
	if len(interfaces[2].IPv4) != 0 {
		t.Errorf("expected third interface to have no IPs, got %v", interfaces[2].IPv4)
	}
}

func TestWicked_EmptyInput(t *testing.T) {
	interfaces, err := Wicked("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(interfaces))
	}
}

func TestWicked_MalformedXML(t *testing.T) {
	files := "lease-eth0-dhcp-ipv4.xml\n"
	xml := "<not-a-lease>broken"
	_, err := Wicked(files, xml)
	if err == nil {
		t.Error("expected error for malformed XML, got nil")
	}
}

func TestWicked_TruncatedXML(t *testing.T) {
	files := "lease-eth0-dhcp-ipv4.xml\n"
	xmlSection := `<lease>
    <family>ipv4</family>
    <type>dhcp</type>
    <state>granted</state>
    <ipv4:dhcp>
        <address>192.168.122.82</address>
    </ipv4:dhcp>`
	_, err := Wicked(files, xmlSection)
	if err == nil {
		t.Error("expected error for truncated XML (missing </lease>), got nil")
	}
}
