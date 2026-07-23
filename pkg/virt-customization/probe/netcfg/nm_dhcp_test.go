package netcfg

import "testing"

func TestNMDhcpLease(t *testing.T) {
	t.Parallel()
	files := "internal-6a0608c2-9d57-46d8-84d1-d2bbf6767acc-enp1s0.lease\ninternal-1f2ea111-624b-32f6-8c93-12b2388516a6-enp1s0.lease\n"
	leases := "# This is private data. Do not parse.\nADDRESS=192.168.122.167\n# This is private data. Do not parse.\nADDRESS=192.168.122.179\n"
	timestamps := "1f2ea111-624b-32f6-8c93-12b2388516a6=1745269160\n6a0608c2-9d57-46d8-84d1-d2bbf6767acc=1745269159\n"

	interfaces, err := NMDhcpLease(files, leases, timestamps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface (deduplicated by name), got %d", len(interfaces))
	}
	iface := interfaces[0]
	if iface.Name != "enp1s0" {
		t.Errorf("expected enp1s0, got %s", iface.Name)
	}
	// UUID 1f2ea111... has timestamp 1745269160 (newer), ADDRESS=192.168.122.179
	if !containsStr(iface.IPv4, "192.168.122.179") {
		t.Errorf("expected IPv4 192.168.122.179 (from newer lease), got %v", iface.IPv4)
	}
	if !iface.DHCP {
		t.Error("expected DHCP=true")
	}
	if iface.Source != "nm-dhcp-lease" {
		t.Errorf("expected source nm-dhcp-lease, got %s", iface.Source)
	}
}

func TestNMDhcpLease_MultipleInterfaces(t *testing.T) {
	t.Parallel()
	files := "internal-aabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee-eth0.lease\ninternal-11112222-2222-3333-4444-555555555555-eth1.lease\n"
	leases := "ADDRESS=10.0.0.1\nADDRESS=10.0.0.2\n"
	timestamps := "aabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee=100\n11112222-2222-3333-4444-555555555555=200\n"

	interfaces, err := NMDhcpLease(files, leases, timestamps)
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

func TestNMDhcpLease_NoTimestampsFile(t *testing.T) {
	t.Parallel()
	files := "internal-aabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee-eth0.lease\n"
	leases := "ADDRESS=10.0.0.1\n"
	timestamps := ""

	interfaces, err := NMDhcpLease(files, leases, timestamps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
	if interfaces[0].Name != "eth0" {
		t.Errorf("expected eth0, got %s", interfaces[0].Name)
	}
	if !containsStr(interfaces[0].IPv4, "10.0.0.1") {
		t.Error("expected IPv4 10.0.0.1")
	}
}

func TestNMDhcpLease_TimestampsWithHeader(t *testing.T) {
	t.Parallel()
	files := "internal-aabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee-eth0.lease\n"
	leases := "ADDRESS=10.0.0.1\n"
	timestamps := "[timestamps]\naabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee=999\n"

	interfaces, err := NMDhcpLease(files, leases, timestamps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(interfaces))
	}
}

func TestNMDhcpLease_EmptyInput(t *testing.T) {
	t.Parallel()
	interfaces, err := NMDhcpLease("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 0 {
		t.Errorf("expected 0 interfaces for empty input, got %d", len(interfaces))
	}
}

func TestNMDhcpLease_NonLeaseFilesIgnored(t *testing.T) {
	t.Parallel()
	files := "timestamps\nsecret_key\ninternal-aabbccdd-bbbb-cccc-dddd-eeeeeeeeeeee-eth0.lease\n"
	leases := "ADDRESS=10.0.0.1\n"
	timestamps := ""

	interfaces, err := NMDhcpLease(files, leases, timestamps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(interfaces) != 1 {
		t.Fatalf("expected 1 interface (non-.lease files ignored), got %d", len(interfaces))
	}
}

func TestParseNMLeaseFilenames(t *testing.T) {
	t.Parallel()
	section := "timestamps\ninternal-6a0608c2-9d57-46d8-84d1-d2bbf6767acc-enp1s0.lease\ninternal-1f2ea111-624b-32f6-8c93-12b2388516a6-ens192.lease\n"
	files := parseNMLeaseFilenames(section)
	if len(files) != 2 {
		t.Fatalf("expected 2 lease files, got %d", len(files))
	}
	if files[0].uuid != "6a0608c2-9d57-46d8-84d1-d2bbf6767acc" {
		t.Errorf("unexpected uuid: %s", files[0].uuid)
	}
	if files[0].iface != "enp1s0" {
		t.Errorf("expected enp1s0, got %s", files[0].iface)
	}
	if files[1].iface != "ens192" {
		t.Errorf("expected ens192, got %s", files[1].iface)
	}
}

func TestParseTimestamps(t *testing.T) {
	t.Parallel()
	section := "[timestamps]\n1f2ea111-624b-32f6-8c93-12b2388516a6=1745269160\n6a0608c2-9d57-46d8-84d1-d2bbf6767acc=1745269159\n\n"
	ts := parseTimestamps(section)
	if len(ts) != 2 {
		t.Fatalf("expected 2 timestamps, got %d", len(ts))
	}
	if ts["1f2ea111-624b-32f6-8c93-12b2388516a6"] != 1745269160 {
		t.Errorf("unexpected timestamp: %d", ts["1f2ea111-624b-32f6-8c93-12b2388516a6"])
	}
	if ts["6a0608c2-9d57-46d8-84d1-d2bbf6767acc"] != 1745269159 {
		t.Errorf("unexpected timestamp: %d", ts["6a0608c2-9d57-46d8-84d1-d2bbf6767acc"])
	}
}

func TestParseNMLeaseAddresses(t *testing.T) {
	t.Parallel()
	section := "# This is private data. Do not parse.\nADDRESS=192.168.122.167\n# This is private data. Do not parse.\nADDRESS=192.168.122.179\n"
	addrs := parseNMLeaseAddresses(section)
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(addrs))
	}
	if addrs[0] != "192.168.122.167" {
		t.Errorf("expected 192.168.122.167, got %s", addrs[0])
	}
	if addrs[1] != "192.168.122.179" {
		t.Errorf("expected 192.168.122.179, got %s", addrs[1])
	}
}
