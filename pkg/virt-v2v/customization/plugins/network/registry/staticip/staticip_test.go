package staticip

import (
	"testing"
)

func TestParseEntries_SingleMAC(t *testing.T) {
	t.Parallel()
	input := "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,"
	result, warnings := ParseEntries(input)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	entries, ok := result["aa-bb-cc-dd-ee-ff"]
	if !ok {
		t.Fatal("expected MAC aa-bb-cc-dd-ee-ff in result")
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IP != "10.0.0.1" {
		t.Errorf("expected IP 10.0.0.1, got %s", entries[0].IP)
	}
	if entries[0].Gateway != "10.0.0.254" {
		t.Errorf("expected gateway 10.0.0.254, got %s", entries[0].Gateway)
	}
	if entries[0].PrefixLength != "24" {
		t.Errorf("expected prefix 24, got %s", entries[0].PrefixLength)
	}
}

func TestParseEntries_MultipleIPs(t *testing.T) {
	t.Parallel()
	input := "aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,_aa:bb:cc:dd:ee:ff:ip:10.0.0.2,10.0.0.254,24,8.8.4.4,"
	result, warnings := ParseEntries(input)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	entries := result["aa-bb-cc-dd-ee-ff"]
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "10.0.0.1" {
		t.Errorf("expected first IP 10.0.0.1, got %s", entries[0].IP)
	}
	if entries[1].IP != "10.0.0.2" {
		t.Errorf("expected second IP 10.0.0.2, got %s", entries[1].IP)
	}
}

func TestParseEntries_SkipsMalformed(t *testing.T) {
	t.Parallel()
	input := "garbage_aa:bb:cc:dd:ee:ff:ip:10.0.0.1,10.0.0.254,24,8.8.8.8,_short:ip:1.2"
	result, warnings := ParseEntries(input)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings for malformed segments, got %d: %v", len(warnings), warnings)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 MAC group, got %d", len(result))
	}
	if _, ok := result["aa-bb-cc-dd-ee-ff"]; !ok {
		t.Error("expected aa-bb-cc-dd-ee-ff in result")
	}
}

func TestBuildComplementaryConfigs_DropsFirstIP(t *testing.T) {
	t.Parallel()
	macMap := map[string][]IPEntry{
		"aa-bb-cc-dd-ee-ff": {
			{IP: "10.0.0.1", Gateway: "10.0.0.254", PrefixLength: "24", DNS: []string{"8.8.8.8"}},
			{IP: "10.0.0.2", Gateway: "10.0.0.254", PrefixLength: "24", DNS: []string{"8.8.8.8"}},
		},
	}
	configs := BuildComplementaryConfigs(macMap)
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].MAC != "aa-bb-cc-dd-ee-ff" {
		t.Errorf("expected MAC aa-bb-cc-dd-ee-ff, got %s", configs[0].MAC)
	}
	if len(configs[0].IPs) != 1 {
		t.Fatalf("expected 1 complementary IP, got %d", len(configs[0].IPs))
	}
	if configs[0].IPs[0].IP != "10.0.0.2" {
		t.Errorf("expected IP 10.0.0.2, got %s", configs[0].IPs[0].IP)
	}
}

func TestBuildComplementaryConfigs_SingleIPSkipped(t *testing.T) {
	t.Parallel()
	macMap := map[string][]IPEntry{
		"aa-bb-cc-dd-ee-ff": {
			{IP: "10.0.0.1", Gateway: "10.0.0.254", PrefixLength: "24", DNS: []string{"8.8.8.8"}},
		},
	}
	configs := BuildComplementaryConfigs(macMap)
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs for single-IP MAC, got %d", len(configs))
	}
}

func TestBuildComplementaryConfigs_SortedByMAC(t *testing.T) {
	t.Parallel()
	macMap := map[string][]IPEntry{
		"cc-dd-ee-ff-00-11": {
			{IP: "10.0.0.1"}, {IP: "10.0.0.2"},
		},
		"aa-bb-cc-dd-ee-ff": {
			{IP: "192.168.1.1"}, {IP: "192.168.1.2"},
		},
	}
	configs := BuildComplementaryConfigs(macMap)
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	if configs[0].MAC != "aa-bb-cc-dd-ee-ff" {
		t.Errorf("expected first MAC aa-bb-cc-dd-ee-ff, got %s", configs[0].MAC)
	}
	if configs[1].MAC != "cc-dd-ee-ff-00-11" {
		t.Errorf("expected second MAC cc-dd-ee-ff-00-11, got %s", configs[1].MAC)
	}
}
