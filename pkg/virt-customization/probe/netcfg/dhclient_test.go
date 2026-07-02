package netcfg

import "testing"

func TestDhclient(t *testing.T) {
	t.Parallel()
	// Two lease blocks for eth0; second has later expire -> wins with IP .82
	section := `lease {
  interface "eth0";
  fixed-address 192.168.122.83;
  option subnet-mask 255.255.255.0;
  expire 3 2025/04/30 17:40:05;
}
lease {
  interface "eth0";
  fixed-address 192.168.122.82;
  option subnet-mask 255.255.255.0;
  expire 3 2025/04/30 17:42:30;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface (same name deduplicated), got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "192.168.122.82") {
		t.Errorf("expected IPv4 192.168.122.82 (latest expire), got %v", interfaces[0].IPv4)
	}
	if !interfaces[0].DHCP {
		t.Error("expected DHCP=true")
	}
	if interfaces[0].Source != "dhclient" {
		t.Errorf("expected source dhclient, got %s", interfaces[0].Source)
	}
}

func TestDhclient_MultipleInterfaces(t *testing.T) {
	t.Parallel()
	section := `lease {
  interface "eth0";
  fixed-address 192.168.122.82;
  expire 3 2025/04/30 17:42:30;
}
lease {
  interface "eth1";
  fixed-address 192.168.122.83;
  expire 3 2025/04/30 17:42:30;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	found := map[string]bool{}
	for _, iface := range interfaces {
		found[iface.Name] = true
	}
	if !found["eth0"] || !found["eth1"] {
		t.Errorf("expected eth0 and eth1, got %v", found)
	}
}

func TestDhclient_LatestExpireWins(t *testing.T) {
	t.Parallel()
	// eth1 has two blocks: older one has IP .82, newer one has IP .83
	section := `lease {
  interface "eth1";
  fixed-address 192.168.122.82;
  expire 3 2024/03/14 12:40:05;
}
lease {
  interface "eth1";
  fixed-address 192.168.122.83;
  expire 3 2025/04/30 17:42:30;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "192.168.122.83") {
		t.Errorf("expected 192.168.122.83 (latest), got %v", interfaces[0].IPv4)
	}
}

func TestDhclient_EmptyInput(t *testing.T) {
	t.Parallel()
	interfaces, err := Dhclient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(interfaces))
	}
}

func TestDhclient_IncompleteBlock(t *testing.T) {
	t.Parallel()
	// Block missing fixed-address should be skipped
	section := `lease {
  interface "eth0";
  expire 3 2025/04/30 17:42:30;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces (incomplete block), got %d", len(interfaces))
	}
}

func TestDhclient_MissingExpire(t *testing.T) {
	t.Parallel()
	// Block with no expire should still be usable (zero time)
	section := `lease {
  interface "eth0";
  fixed-address 10.0.0.1;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if !containsStr(interfaces[0].IPv4, "10.0.0.1") {
		t.Error("expected IPv4 10.0.0.1")
	}
}

func TestParseDhclientBlocks(t *testing.T) {
	t.Parallel()
	section := `lease {
  interface "eth0";
  fixed-address 192.168.122.82;
  option subnet-mask 255.255.255.0;
  option routers 192.168.122.1;
  expire 3 2025/04/30 17:42:30;
}
`
	blocks := parseDhclientBlocks(section)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].iface != "eth0" {
		t.Errorf("expected eth0, got %s", blocks[0].iface)
	}
	if blocks[0].ip != "192.168.122.82" {
		t.Errorf("expected 192.168.122.82, got %s", blocks[0].ip)
	}
	if blocks[0].expire.IsZero() {
		t.Error("expected non-zero expire time")
	}
}

func TestDhclient_ConcatenatedFiles(t *testing.T) {
	t.Parallel()
	// Simulates two files concatenated (as catGlob would produce)
	section := `lease {
  interface "eth0";
  fixed-address 192.168.122.82;
  expire 3 2025/04/30 17:42:30;
}
lease {
  interface "eth0";
  fixed-address 192.168.122.83;
  expire 3 2025/04/30 17:40:05;
}
lease {
  interface "eth1";
  fixed-address 192.168.122.84;
  expire 3 2025/04/30 17:42:30;
}
`
	interfaces, err := Dhclient(section)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(interfaces))
	}
	for _, iface := range interfaces {
		switch iface.Name {
		case "eth0":
			if !containsStr(iface.IPv4, "192.168.122.82") {
				t.Errorf("expected eth0 to have 192.168.122.82 (later expire), got %v", iface.IPv4)
			}
		case "eth1":
			if !containsStr(iface.IPv4, "192.168.122.84") {
				t.Errorf("expected eth1 to have 192.168.122.84, got %v", iface.IPv4)
			}
		default:
			t.Errorf("unexpected interface: %s", iface.Name)
		}
	}
}
