package ssh

import "testing"

func TestParseSSHDConfig_Basic(t *testing.T) {
	t.Parallel()
	config := `# SSH config
Port 2222
PermitRootLogin prohibit-password
PasswordAuthentication no
`
	result := ParseSSHDConfig(config)
	if result.Port != 2222 {
		t.Errorf("expected port 2222, got %d", result.Port)
	}
	if result.PermitRootLogin != "prohibit-password" {
		t.Errorf("expected prohibit-password, got %s", result.PermitRootLogin)
	}
	if result.PasswordAuthentication == nil {
		t.Fatal("expected PasswordAuthentication to be set")
	}
	if *result.PasswordAuthentication {
		t.Error("expected PasswordAuthentication=false")
	}
}

func TestParseSSHDConfig_FirstMatchWins(t *testing.T) {
	t.Parallel()
	config := `PermitRootLogin yes
PermitRootLogin no
Port 22
Port 2222
PasswordAuthentication yes
PasswordAuthentication no
`
	result := ParseSSHDConfig(config)
	if result.PermitRootLogin != "yes" {
		t.Errorf("expected first match 'yes', got %s", result.PermitRootLogin)
	}
	if result.Port != 22 {
		t.Errorf("expected first match port 22, got %d", result.Port)
	}
	if result.PasswordAuthentication == nil || !*result.PasswordAuthentication {
		t.Error("expected first match PasswordAuthentication=true")
	}
}

func TestParseSSHDConfig_DefaultPort(t *testing.T) {
	t.Parallel()
	config := `PermitRootLogin yes
`
	result := ParseSSHDConfig(config)
	if result.Port != 0 {
		t.Errorf("expected port 0 (unset), got %d", result.Port)
	}
}

func TestParseSSHDConfig_Comments(t *testing.T) {
	t.Parallel()
	config := `# PermitRootLogin yes
#Port 22
  # PasswordAuthentication no
PermitRootLogin no
`
	result := ParseSSHDConfig(config)
	if result.PermitRootLogin != "no" {
		t.Errorf("expected 'no' (comments skipped), got %s", result.PermitRootLogin)
	}
	if result.Port != 0 {
		t.Errorf("expected port 0 (commented out), got %d", result.Port)
	}
}

func TestParseSSHDConfig_EmptyInput(t *testing.T) {
	t.Parallel()
	result := ParseSSHDConfig("")
	if result.PermitRootLogin != "" {
		t.Errorf("expected empty PermitRootLogin, got %s", result.PermitRootLogin)
	}
	if result.PasswordAuthentication != nil {
		t.Error("expected nil PasswordAuthentication")
	}
	if result.Port != 0 {
		t.Errorf("expected port 0, got %d", result.Port)
	}
}

func TestParseSSHDConfig_CaseInsensitive(t *testing.T) {
	t.Parallel()
	config := `PERMITROOTLOGIN yes
PASSWORDAUTHENTICATION yes
PORT 3333
`
	result := ParseSSHDConfig(config)
	if result.PermitRootLogin != "yes" {
		t.Errorf("expected yes (case insensitive), got %s", result.PermitRootLogin)
	}
	if result.Port != 3333 {
		t.Errorf("expected 3333, got %d", result.Port)
	}
	if result.PasswordAuthentication == nil || !*result.PasswordAuthentication {
		t.Error("expected PasswordAuthentication=true")
	}
}

func TestParseSSHDConfig_ConcatenatedWithDropins(t *testing.T) {
	t.Parallel()
	// Main config first, then drop-in; first match wins
	config := `PermitRootLogin yes
Port 22
` + `# Drop-in from /etc/ssh/sshd_config.d/50-cloud-init.conf
PermitRootLogin no
PasswordAuthentication no
`
	result := ParseSSHDConfig(config)
	if result.PermitRootLogin != "yes" {
		t.Errorf("expected 'yes' from main config, got %s", result.PermitRootLogin)
	}
	if result.Port != 22 {
		t.Errorf("expected 22, got %d", result.Port)
	}
	if result.PasswordAuthentication == nil || *result.PasswordAuthentication {
		t.Error("expected PasswordAuthentication=false from drop-in (not set in main)")
	}
}

func TestHostKeyTypeFromFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"ssh_host_rsa_key", "rsa"},
		{"ssh_host_ed25519_key", "ed25519"},
		{"ssh_host_ecdsa_key", "ecdsa"},
		{"ssh_host_dsa_key", "dsa"},
		{"ssh_host_rsa_key.pub", "rsa"},
	}
	for _, tt := range tests {
		got := HostKeyTypeFromFilename(tt.input)
		if got != tt.want {
			t.Errorf("HostKeyTypeFromFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
