package cloudinit

import "testing"

func TestParseCloudCfg_DatasourceList(t *testing.T) {
	t.Parallel()
	cfg := `datasource_list:
  - NoCloud
  - ConfigDrive
  - OpenStack
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.DatasourceList) != 3 {
		t.Fatalf("expected 3 datasources, got %d", len(result.DatasourceList))
	}
	if result.DatasourceList[0] != "NoCloud" {
		t.Errorf("expected NoCloud, got %s", result.DatasourceList[0])
	}
	if result.DatasourceList[2] != "OpenStack" {
		t.Errorf("expected OpenStack, got %s", result.DatasourceList[2])
	}
}

func TestParseCloudCfg_NetworkDisabled(t *testing.T) {
	t.Parallel()
	cfg := `network:
  config: disabled
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.NetworkConfigDisabled {
		t.Error("expected NetworkConfigDisabled to be true")
	}
}

func TestParseCloudCfg_NetworkNotDisabled(t *testing.T) {
	t.Parallel()
	cfg := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NetworkConfigDisabled {
		t.Error("expected NetworkConfigDisabled to be false")
	}
}

func TestParseCloudCfg_NoNetworkKey(t *testing.T) {
	t.Parallel()
	cfg := `datasource_list:
  - EC2
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NetworkConfigDisabled {
		t.Error("expected NetworkConfigDisabled false when network key absent")
	}
}

func TestParseCloudCfg_MultipleDocuments(t *testing.T) {
	t.Parallel()
	// Simulates cloud.cfg + cloud.cfg.d/99-datasource.cfg concatenated
	cfg := `datasource_list:
  - EC2
  - None
---
datasource_list:
  - NoCloud
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Later document overrides
	if len(result.DatasourceList) != 1 || result.DatasourceList[0] != "NoCloud" {
		t.Errorf("expected [NoCloud] from override, got %v", result.DatasourceList)
	}
}

func TestParseCloudCfg_EmptyInput(t *testing.T) {
	t.Parallel()
	result, err := ParseCloudCfg("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.DatasourceList) != 0 {
		t.Errorf("expected empty DatasourceList, got %v", result.DatasourceList)
	}
}

func TestParseCloudCfg_InvalidYAML(t *testing.T) {
	t.Parallel()
	_, err := ParseCloudCfg(":\n  bad: [yaml")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseCloudCfg_FullConfig(t *testing.T) {
	t.Parallel()
	cfg := `users:
  - default
disable_root: true
datasource_list:
  - VMware
  - OVF
  - None
network:
  config: disabled
ssh_authorized_keys:
  - ssh-rsa AAAA...
`
	result, err := ParseCloudCfg(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.DatasourceList) != 3 {
		t.Fatalf("expected 3 datasources, got %d", len(result.DatasourceList))
	}
	if result.DatasourceList[0] != "VMware" {
		t.Errorf("expected VMware, got %s", result.DatasourceList[0])
	}
	if !result.NetworkConfigDisabled {
		t.Error("expected NetworkConfigDisabled to be true")
	}
}

func TestParseDatasourceFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"DataSourceNoCloud [seed=/dev/sr0][dsmode=net]\n", "NoCloud"},
		{"DataSourceConfigDrive\n", "ConfigDrive"},
		{"DataSourceEC2Local\n", "EC2Local"},
		{"DataSourceOpenStackLocal [ds=OpenStackLocal,ver=2]\n", "OpenStackLocal"},
		{"", ""},
		{"\n", ""},
	}
	for _, tt := range tests {
		got := ParseDatasourceFile(tt.input)
		if got != tt.want {
			t.Errorf("ParseDatasourceFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
