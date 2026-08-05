package commit

import (
	"errors"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// mockGuestHandle records file operations for assertions.
type mockGuestHandle struct {
	mkdirs  []string
	uploads []uploadCall
	writes  []writeCall
	chmods  []chmodCall
	failOn  string // method name to fail on (e.g. "MkdirP", "Upload", "Write", "Chmod")
	failErr error
}

type uploadCall struct{ local, guest string }
type writeCall struct {
	path    string
	content []byte
}
type chmodCall struct {
	mode int
	path string
}

func (m *mockGuestHandle) IsDir(string) (bool, error)          { return false, nil }
func (m *mockGuestHandle) IsFile(string) (bool, error)         { return false, nil }
func (m *mockGuestHandle) Cat(string) (string, error)          { return "", nil }
func (m *mockGuestHandle) GlobExpand(string) ([]string, error) { return nil, nil }
func (m *mockGuestHandle) Ls(string) ([]string, error)         { return nil, nil }
func (m *mockGuestHandle) ReadFile(string) ([]byte, error)     { return nil, nil }
func (m *mockGuestHandle) Shutdown() error                     { return nil }
func (m *mockGuestHandle) Close()                              {}

func (m *mockGuestHandle) MkdirP(path string) error {
	if m.failOn == "MkdirP" {
		return m.failErr
	}
	m.mkdirs = append(m.mkdirs, path)
	return nil
}

func (m *mockGuestHandle) Upload(local, guest string) error {
	if m.failOn == "Upload" {
		return m.failErr
	}
	m.uploads = append(m.uploads, uploadCall{local, guest})
	return nil
}

func (m *mockGuestHandle) Write(path string, content []byte) error {
	if m.failOn == "Write" {
		return m.failErr
	}
	m.writes = append(m.writes, writeCall{path, content})
	return nil
}

func (m *mockGuestHandle) Chmod(mode int, path string) error {
	if m.failOn == "Chmod" {
		return m.failErr
	}
	m.chmods = append(m.chmods, chmodCall{mode, path})
	return nil
}

var _ api.GuestHandle = (*mockGuestHandle)(nil)

func TestFiles_EmptyActions(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	err := Files(g, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(g.mkdirs) > 0 || len(g.uploads) > 0 || len(g.writes) > 0 {
		t.Error("expected no operations for empty actions")
	}
}

func TestFiles_EmptySlice(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	err := Files(g, []api.FileAction{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

func TestFiles_Upload(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{
			Type:      api.ActionUpload,
			LocalPath: "/tmp/script.bat",
			GuestPath: "/Program Files/Guestfs/Firstboot/scripts/script.bat",
		},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.uploads) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(g.uploads))
	}
	if g.uploads[0].local != "/tmp/script.bat" {
		t.Errorf("upload local = %q", g.uploads[0].local)
	}
	if g.uploads[0].guest != "/Program Files/Guestfs/Firstboot/scripts/script.bat" {
		t.Errorf("upload guest = %q", g.uploads[0].guest)
	}
}

func TestFiles_Write(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	content := []byte("SUBSYSTEM==\"net\",ACTION==\"add\"\n")
	actions := []api.FileAction{
		{
			Type:      api.ActionWrite,
			GuestPath: "/etc/udev/rules.d/70-persistent-net.rules",
			Content:   content,
		},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.writes) != 1 {
		t.Fatalf("expected 1 write, got %d", len(g.writes))
	}
	if g.writes[0].path != "/etc/udev/rules.d/70-persistent-net.rules" {
		t.Errorf("write path = %q", g.writes[0].path)
	}
	if string(g.writes[0].content) != string(content) {
		t.Errorf("write content mismatch")
	}
}

func TestFiles_Chmod(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{
			Type:        api.ActionWrite,
			GuestPath:   "/etc/udev/rules.d/70-persistent-net.rules",
			Content:     []byte("rule\n"),
			Permissions: "0644",
		},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.chmods) != 1 {
		t.Fatalf("expected 1 chmod, got %d", len(g.chmods))
	}
	if g.chmods[0].mode != 0644 {
		t.Errorf("chmod mode = %o, want 0644", g.chmods[0].mode)
	}
	if g.chmods[0].path != "/etc/udev/rules.d/70-persistent-net.rules" {
		t.Errorf("chmod path = %q", g.chmods[0].path)
	}
}

func TestFiles_MultipleActions(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{Type: api.ActionUpload, LocalPath: "/a.sh", GuestPath: "/b.sh"},
		{Type: api.ActionWrite, GuestPath: "/c.txt", Content: []byte("hello")},
		{Type: api.ActionUpload, LocalPath: "/d.bat", GuestPath: "/e.bat", Permissions: "0755"},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.uploads) != 2 {
		t.Errorf("expected 2 uploads, got %d", len(g.uploads))
	}
	if len(g.writes) != 1 {
		t.Errorf("expected 1 write, got %d", len(g.writes))
	}
	if len(g.chmods) != 1 {
		t.Errorf("expected 1 chmod, got %d", len(g.chmods))
	}
	if g.chmods[0].mode != 0755 {
		t.Errorf("chmod mode = %o, want 0755", g.chmods[0].mode)
	}
}

func TestFiles_UnsupportedActionType(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{Type: "bogus", GuestPath: "/etc/test"},
	}
	err := Files(g, actions)
	if err == nil {
		t.Fatal("expected error for unsupported action type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected error mentioning 'unsupported', got: %v", err)
	}
}

func TestFiles_InvalidPermissions(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{Type: api.ActionWrite, GuestPath: "/etc/test", Content: []byte("data"), Permissions: "abc"},
	}
	err := Files(g, actions)
	if err == nil {
		t.Fatal("expected error for invalid permissions, got nil")
	}
	if !strings.Contains(err.Error(), "invalid permissions") {
		t.Errorf("expected error mentioning 'invalid permissions', got: %v", err)
	}
}

func TestFiles_MkdirP(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{Type: api.ActionWrite, GuestPath: "/Program Files/Guestfs/Firstboot/scripts/100_config.ps1", Content: []byte("echo hi")},
		{Type: api.ActionWrite, GuestPath: "/Program Files/Guestfs/Firstboot/scripts/200_restore.ps1", Content: []byte("echo bye")},
		{Type: api.ActionWrite, GuestPath: "/etc/udev/rules.d/70-persistent-net.rules", Content: []byte("rule")},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Deduplicated: only 2 unique dirs
	if len(g.mkdirs) != 2 {
		t.Errorf("expected 2 deduplicated mkdirs, got %d: %v", len(g.mkdirs), g.mkdirs)
	}

	containsDir := func(dir string) bool {
		for _, d := range g.mkdirs {
			if d == dir {
				return true
			}
		}
		return false
	}
	if !containsDir("/Program Files/Guestfs/Firstboot/scripts") {
		t.Errorf("expected mkdir for firstboot scripts dir, got: %v", g.mkdirs)
	}
	if !containsDir("/etc/udev/rules.d") {
		t.Errorf("expected mkdir for udev dir, got: %v", g.mkdirs)
	}
}

func TestFiles_MkdirBeforeWrite(t *testing.T) {
	t.Parallel()
	// mkdirs are called before writes/uploads since Files does dirs first, then actions
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{Type: api.ActionUpload, LocalPath: "/a", GuestPath: "/deep/dir/file.txt"},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.mkdirs) < 1 {
		t.Fatal("expected at least one mkdir")
	}
	if len(g.uploads) < 1 {
		t.Fatal("expected at least one upload")
	}
}

func TestFiles_UploadWithPermissions(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{}
	actions := []api.FileAction{
		{
			Type:        api.ActionUpload,
			LocalPath:   "/tmp/myscript.sh",
			GuestPath:   "/usr/local/bin/myscript.sh",
			Permissions: "0755",
		},
	}
	err := Files(g, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.uploads) != 1 || g.uploads[0].guest != "/usr/local/bin/myscript.sh" {
		t.Errorf("unexpected uploads: %v", g.uploads)
	}
	if len(g.chmods) != 1 || g.chmods[0].mode != 0755 {
		t.Errorf("unexpected chmods: %v", g.chmods)
	}
}

func TestFiles_UploadError(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{failOn: "Upload", failErr: errors.New("upload failed")}
	actions := []api.FileAction{
		{Type: api.ActionUpload, LocalPath: "/a", GuestPath: "/b"},
	}
	err := Files(g, actions)
	if err == nil {
		t.Fatal("expected error from Upload")
	}
	if !strings.Contains(err.Error(), "upload failed") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestFiles_WriteError(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{failOn: "Write", failErr: errors.New("write failed")}
	actions := []api.FileAction{
		{Type: api.ActionWrite, GuestPath: "/etc/test", Content: []byte("data")},
	}
	err := Files(g, actions)
	if err == nil {
		t.Fatal("expected error from Write")
	}
	if !strings.Contains(err.Error(), "write failed") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestFiles_MkdirError(t *testing.T) {
	t.Parallel()
	g := &mockGuestHandle{failOn: "MkdirP", failErr: errors.New("mkdir failed")}
	actions := []api.FileAction{
		{Type: api.ActionUpload, LocalPath: "/a", GuestPath: "/deep/dir/file.txt"},
	}
	err := Files(g, actions)
	if err == nil {
		t.Fatal("expected error from MkdirP")
	}
	if !strings.Contains(err.Error(), "mkdir failed") {
		t.Errorf("expected wrapped error, got %v", err)
	}
}
