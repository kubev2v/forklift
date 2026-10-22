package hyperv

import "testing"

func TestSMBMaxChannels(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		want     int
	}{
		{"empty settings", map[string]string{}, 0},
		{"not set", map[string]string{"other": "val"}, 0},
		{"zero", map[string]string{SettingSMBMaxChannels: "0"}, 0},
		{"valid 4", map[string]string{SettingSMBMaxChannels: "4"}, 4},
		{"valid 1", map[string]string{SettingSMBMaxChannels: "1"}, 1},
		{"max 16", map[string]string{SettingSMBMaxChannels: "16"}, 16},
		{"clamped to 16", map[string]string{SettingSMBMaxChannels: "32"}, 16},
		{"negative", map[string]string{SettingSMBMaxChannels: "-1"}, 0},
		{"non-numeric", map[string]string{SettingSMBMaxChannels: "abc"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SMBMaxChannels(tt.settings)
			if got != tt.want {
				t.Errorf("SMBMaxChannels() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSMBMountOptions(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		wantNil  bool
		wantOpt  string
	}{
		{"disabled by default", map[string]string{}, true, ""},
		{"disabled when 0", map[string]string{SettingSMBMaxChannels: "0"}, true, ""},
		{"enabled with 4", map[string]string{SettingSMBMaxChannels: "4"}, false, "max_channels=4"},
		{"clamped to 16", map[string]string{SettingSMBMaxChannels: "99"}, false, "max_channels=16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SMBMountOptions(tt.settings)
			if tt.wantNil {
				if opts != nil {
					t.Errorf("expected nil, got %v", opts)
				}
				return
			}
			if len(opts) != 1 || opts[0] != tt.wantOpt {
				t.Errorf("expected [%s], got %v", tt.wantOpt, opts)
			}
		})
	}
}
