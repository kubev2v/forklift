package commit

import (
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

func TestBuildScript_Upload(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{
			Type:      api.ActionUpload,
			LocalPath: "/tmp/script.bat",
			GuestPath: "/Program Files/Guestfs/Firstboot/scripts/script.bat",
		},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `upload "/tmp/script.bat" "/Program Files/Guestfs/Firstboot/scripts/script.bat"`) {
		t.Errorf("expected quoted upload command, got: %s", script)
	}
}

func TestBuildScript_Write(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{
			Type:      api.ActionWrite,
			GuestPath: "/etc/udev/rules.d/70-persistent-net.rules",
			Content:   []byte("SUBSYSTEM==\"net\",ACTION==\"add\",ATTR{address}==\"00:11:22:33:44:55\",NAME=\"eth0\"\n"),
		},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `write "/etc/udev/rules.d/70-persistent-net.rules"`) {
		t.Errorf("expected quoted write command, got: %s", script)
	}
	if !strings.Contains(script, "SUBSYSTEM") {
		t.Errorf("expected content in script, got: %s", script)
	}
}

func TestBuildScript_Chmod(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{
			Type:        api.ActionWrite,
			GuestPath:   "/etc/udev/rules.d/70-persistent-net.rules",
			Content:     []byte("rule\n"),
			Permissions: "0644",
		},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `chmod 0644 "/etc/udev/rules.d/70-persistent-net.rules"`) {
		t.Errorf("expected quoted chmod, got: %s", script)
	}
}

func TestBuildScript_MultipleActions(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{Type: api.ActionUpload, LocalPath: "/a.sh", GuestPath: "/b.sh"},
		{Type: api.ActionWrite, GuestPath: "/c.txt", Content: []byte("hello")},
		{Type: api.ActionUpload, LocalPath: "/d.bat", GuestPath: "/e.bat", Permissions: "0755"},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `upload "/a.sh" "/b.sh"`) {
		t.Error(`expected upload "/a.sh" "/b.sh"`)
	}
	if !strings.Contains(script, `write "/c.txt"`) {
		t.Error(`expected write "/c.txt"`)
	}
	if !strings.Contains(script, `upload "/d.bat" "/e.bat"`) {
		t.Error(`expected upload "/d.bat" "/e.bat"`)
	}
	if !strings.Contains(script, `chmod 0755 "/e.bat"`) {
		t.Error(`expected chmod 0755 "/e.bat"`)
	}
}

func TestBuildScript_Empty(t *testing.T) {
	t.Parallel()
	script, err := buildScript(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script != "" {
		t.Errorf("expected empty script, got: %s", script)
	}
}

func TestBuildScript_WriteSpecialChars(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\twith\ttabs\nquote\"here\nbackslash\\\n"
	actions := []api.FileAction{
		{
			Type:      api.ActionWrite,
			GuestPath: "/etc/test.conf",
			Content:   []byte(content),
		},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `write "/etc/test.conf"`) {
		t.Errorf("expected quoted write command, got: %s", script)
	}
	if !strings.Contains(script, `\n`) {
		t.Errorf("expected escaped newline in script, got: %s", script)
	}
	if !strings.Contains(script, `\t`) {
		t.Errorf("expected escaped tab in script, got: %s", script)
	}
	if !strings.Contains(script, `\"`) {
		t.Errorf("expected escaped quote in script, got: %s", script)
	}
	if !strings.Contains(script, `\\`) {
		t.Errorf("expected escaped backslash in script, got: %s", script)
	}
}

func TestBuildScript_UploadWithPermissions(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{
			Type:        api.ActionUpload,
			LocalPath:   "/tmp/myscript.sh",
			GuestPath:   "/usr/local/bin/myscript.sh",
			Permissions: "0755",
		},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `upload "/tmp/myscript.sh" "/usr/local/bin/myscript.sh"`) {
		t.Errorf("expected quoted upload command, got: %s", script)
	}
	if !strings.Contains(script, `chmod 0755 "/usr/local/bin/myscript.sh"`) {
		t.Errorf("expected quoted chmod after upload, got: %s", script)
	}
}

func TestBuildScript_EmptySlice(t *testing.T) {
	t.Parallel()
	script, err := buildScript([]api.FileAction{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if script != "" {
		t.Errorf("expected empty script for empty slice, got: %s", script)
	}
}

func TestBuildScript_UnsupportedActionType(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{Type: "bogus", GuestPath: "/etc/test"},
	}
	_, err := buildScript(actions)
	if err == nil {
		t.Fatal("expected error for unsupported action type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected error mentioning 'unsupported', got: %v", err)
	}
}

func TestBuildScript_InvalidPermissions(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{Type: api.ActionWrite, GuestPath: "/etc/test", Content: []byte("data"), Permissions: "abc"},
	}
	_, err := buildScript(actions)
	if err == nil {
		t.Fatal("expected error for invalid permissions, got nil")
	}
	if !strings.Contains(err.Error(), "invalid permissions") {
		t.Errorf("expected error mentioning 'invalid permissions', got: %v", err)
	}
}

func TestBuildScript_MkdirP(t *testing.T) {
	t.Parallel()
	actions := []api.FileAction{
		{Type: api.ActionWrite, GuestPath: "/Program Files/Guestfs/Firstboot/scripts/100_config.ps1", Content: []byte("echo hi")},
		{Type: api.ActionWrite, GuestPath: "/Program Files/Guestfs/Firstboot/scripts/200_restore.ps1", Content: []byte("echo bye")},
		{Type: api.ActionWrite, GuestPath: "/etc/udev/rules.d/70-persistent-net.rules", Content: []byte("rule")},
	}
	script, err := buildScript(actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(script, `mkdir-p "/Program Files/Guestfs/Firstboot/scripts"`) {
		t.Errorf("expected quoted mkdir-p for firstboot scripts dir, got: %s", script)
	}
	if !strings.Contains(script, `mkdir-p "/etc/udev/rules.d"`) {
		t.Errorf("expected quoted mkdir-p for udev dir, got: %s", script)
	}
	mkdirIdx := strings.Index(script, "mkdir-p")
	writeIdx := strings.Index(script, "write")
	if mkdirIdx >= writeIdx {
		t.Errorf("expected mkdir-p before write commands, got: %s", script)
	}
	if strings.Count(script, `mkdir-p "/Program Files/Guestfs/Firstboot/scripts"`) != 1 {
		t.Errorf("expected deduplicated mkdir-p, got: %s", script)
	}
}
