package console

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

func TestParseGrubDefaults_SerialConsole(t *testing.T) {
	t.Parallel()
	grub := `GRUB_TIMEOUT=5
GRUB_CMDLINE_LINUX="console=ttyS0,115200n8 console=tty0 crashkernel=auto"
GRUB_CMDLINE_LINUX_DEFAULT="quiet"
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 2 {
		t.Fatalf("expected 2 consoles, got %d", len(consoles))
	}
	if consoles[0].Device != "ttyS0" {
		t.Errorf("expected ttyS0, got %s", consoles[0].Device)
	}
	if consoles[0].Baud != "115200n8" {
		t.Errorf("expected 115200n8, got %s", consoles[0].Baud)
	}
	if consoles[1].Device != "tty0" {
		t.Errorf("expected tty0, got %s", consoles[1].Device)
	}
	if consoles[1].Baud != "" {
		t.Errorf("expected empty baud for tty0, got %s", consoles[1].Baud)
	}
}

func TestParseGrubDefaults_NoConsole(t *testing.T) {
	t.Parallel()
	grub := `GRUB_TIMEOUT=5
GRUB_CMDLINE_LINUX="crashkernel=auto rd.lvm.lv=centos/root"
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 0 {
		t.Errorf("expected 0 consoles, got %d", len(consoles))
	}
}

func TestParseGrubDefaults_SingleQuotes(t *testing.T) {
	t.Parallel()
	grub := `GRUB_CMDLINE_LINUX='console=ttyS0,9600'
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 1 {
		t.Fatalf("expected 1 console, got %d", len(consoles))
	}
	if consoles[0].Device != "ttyS0" {
		t.Errorf("expected ttyS0, got %s", consoles[0].Device)
	}
	if consoles[0].Baud != "9600" {
		t.Errorf("expected 9600, got %s", consoles[0].Baud)
	}
}

func TestParseGrubDefaults_Comments(t *testing.T) {
	t.Parallel()
	grub := `# GRUB_CMDLINE_LINUX="console=ttyS0"
GRUB_CMDLINE_LINUX="quiet"
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 0 {
		t.Errorf("expected 0 consoles (commented line), got %d", len(consoles))
	}
}

func TestParseGrubDefaults_EmptyInput(t *testing.T) {
	t.Parallel()
	consoles := ParseGrubDefaults("")
	if len(consoles) != 0 {
		t.Errorf("expected 0 consoles, got %d", len(consoles))
	}
}

func TestParseGrubDefaults_DefaultKey(t *testing.T) {
	t.Parallel()
	grub := `GRUB_CMDLINE_LINUX=""
GRUB_CMDLINE_LINUX_DEFAULT="console=ttyAMA0,115200"
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 1 {
		t.Fatalf("expected 1 console from DEFAULT, got %d", len(consoles))
	}
	if consoles[0].Device != "ttyAMA0" {
		t.Errorf("expected ttyAMA0 (ARM), got %s", consoles[0].Device)
	}
}

func TestParseGrubDefaults_OtherKeysIgnored(t *testing.T) {
	t.Parallel()
	grub := `GRUB_TIMEOUT=5
GRUB_DEFAULT=0
GRUB_TERMINAL="serial console"
GRUB_SERIAL_COMMAND="serial --speed=115200"
`
	consoles := ParseGrubDefaults(grub)
	if len(consoles) != 0 {
		t.Errorf("expected 0 consoles (no CMDLINE keys), got %d", len(consoles))
	}
}

func TestParseSerialGettyUnits(t *testing.T) {
	t.Parallel()
	entries := []string{
		"getty@tty1.service",
		"serial-getty@ttyS0.service",
		"serial-getty@ttyS1.service",
		"remote-fs.target",
	}
	devices := ParseSerialGettyUnits(entries)
	if len(devices) != 2 {
		t.Fatalf("expected 2 serial devices, got %d", len(devices))
	}
	if devices[0] != "ttyS0" {
		t.Errorf("expected ttyS0, got %s", devices[0])
	}
	if devices[1] != "ttyS1" {
		t.Errorf("expected ttyS1, got %s", devices[1])
	}
}

func TestParseSerialGettyUnits_Empty(t *testing.T) {
	t.Parallel()
	devices := ParseSerialGettyUnits(nil)
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestParseSerialGettyUnits_NoSerialGetty(t *testing.T) {
	t.Parallel()
	entries := []string{"getty@tty1.service", "getty@tty2.service"}
	devices := ParseSerialGettyUnits(entries)
	if len(devices) != 0 {
		t.Errorf("expected 0 serial devices, got %d", len(devices))
	}
}

func TestConsoleInfo_HasSerialConsole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info api.ConsoleInfo
		want bool
	}{
		{"grub console", api.ConsoleInfo{SerialConsoles: []api.ConsoleDevice{{Device: "ttyS0"}}}, true},
		{"getty unit", api.ConsoleInfo{SerialGettyDevices: []string{"ttyS0"}}, true},
		{"both", api.ConsoleInfo{SerialConsoles: []api.ConsoleDevice{{Device: "ttyS0"}}, SerialGettyDevices: []string{"ttyS0"}}, true},
		{"neither", api.ConsoleInfo{}, false},
	}
	for _, tt := range tests {
		if got := tt.info.HasSerialConsole(); got != tt.want {
			t.Errorf("%s: HasSerialConsole() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
