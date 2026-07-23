package probe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// testHandle implements api.GuestHandle for testing.
type testHandle struct {
	dirs   map[string]bool
	files  map[string]string
	globs  map[string][]string
	lsData map[string][]string
}

func newTestHandle() *testHandle {
	return &testHandle{
		dirs:   make(map[string]bool),
		files:  make(map[string]string),
		globs:  make(map[string][]string),
		lsData: make(map[string][]string),
	}
}

func (h *testHandle) IsDir(path string) (bool, error) { return h.dirs[path], nil }
func (h *testHandle) IsFile(path string) (bool, error) {
	_, ok := h.files[path]
	return ok, nil
}
func (h *testHandle) Cat(path string) (string, error) {
	c, ok := h.files[path]
	if !ok {
		return "", fmt.Errorf("file not found: %s", path)
	}
	return c, nil
}
func (h *testHandle) ReadFile(path string) ([]byte, error) {
	c, err := h.Cat(path)
	return []byte(c), err
}
func (h *testHandle) GlobExpand(pattern string) ([]string, error) {
	return h.globs[pattern], nil
}
func (h *testHandle) Ls(dir string) ([]string, error) { return h.lsData[dir], nil }
func (h *testHandle) MkdirP(string) error             { return nil }
func (h *testHandle) Upload(string, string) error     { return nil }
func (h *testHandle) Write(string, []byte) error      { return nil }
func (h *testHandle) Chmod(int, string) error         { return nil }
func (h *testHandle) Shutdown() error                 { return nil }
func (h *testHandle) Close()                          {}

var _ api.GuestHandle = (*testHandle)(nil)

// --- Detection tests ---

func TestDetect_Windows(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.dirs["/Windows/System32"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if guest.OS.Family != api.OSFamilyWindows {
		t.Errorf("expected Windows, got %s", guest.OS.Family)
	}
}

func TestDetect_Linux(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=rhel\n"
	h.dirs["/etc/sysconfig/network-scripts"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if guest.OS.Family != api.OSFamilyLinux {
		t.Errorf("expected Linux, got %s", guest.OS.Family)
	}
	if !guest.UsesIfcfg {
		t.Error("expected UsesIfcfg to be true")
	}
}

func TestDetect_Unknown(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if guest.OS.Family != api.OSFamilyUnknown {
		t.Errorf("expected Unknown, got %s", guest.OS.Family)
	}
}

func TestDetect_IfcfgSuse(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=sles\n"
	h.dirs["/etc/sysconfig/network"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesIfcfgSuse {
		t.Error("expected UsesIfcfgSuse to be true")
	}
}

func TestDetect_NetworkManager(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=fedora\n"
	h.dirs["/etc/NetworkManager/system-connections"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesNetworkManager {
		t.Error("expected UsesNetworkManager to be true")
	}
}

func TestDetect_Netplan(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=ubuntu\n"
	h.dirs["/etc/netplan"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesNetplan {
		t.Error("expected UsesNetplan to be true")
	}
}

func TestDetect_Wicked(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=sles\n"
	h.dirs["/etc/wicked"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesWicked {
		t.Error("expected UsesWicked to be true")
	}
}

func TestDetect_WickedVar(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=sles\n"
	h.dirs["/var/lib/wicked"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesWicked {
		t.Error("expected UsesWicked to be true")
	}
}

func TestDetect_Ifquery(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=debian\n"
	h.files["/etc/network/interfaces"] = "auto eth0\n"
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesIfquery {
		t.Error("expected UsesIfquery to be true")
	}
}

func TestDetect_InterfacesD(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=debian\n"
	h.dirs["/etc/network/interfaces.d"] = true
	guest := &api.GuestInfo{}
	if err := detect(h, guest); err != nil {
		t.Fatal(err)
	}
	if !guest.UsesInterfacesD {
		t.Error("expected UsesInterfacesD to be true")
	}
}

// --- OsRelease parsing tests ---

func TestParseOsRelease_RHEL(t *testing.T) {
	t.Parallel()
	content := "NAME=\"Red Hat Enterprise Linux\"\nID=rhel\nVERSION_ID=\"9.2\"\n"
	guest := &api.GuestInfo{}
	if err := parseOsRelease(content, guest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guest.OS.Distro != "rhel" {
		t.Errorf("expected rhel, got %s", guest.OS.Distro)
	}
	if guest.OS.Version != "9.2" {
		t.Errorf("expected 9.2, got %s", guest.OS.Version)
	}
}

func TestParseOsRelease_Ubuntu(t *testing.T) {
	t.Parallel()
	content := "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"
	guest := &api.GuestInfo{}
	if err := parseOsRelease(content, guest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guest.OS.Distro != "ubuntu" {
		t.Errorf("expected ubuntu, got %s", guest.OS.Distro)
	}
	if guest.OS.Version != "22.04" {
		t.Errorf("expected 22.04, got %s", guest.OS.Version)
	}
}

// --- Guest integration tests ---

func TestGuest_Windows(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.dirs["/Windows/System32"] = true
	guest, err := Guest(h)
	if err != nil {
		t.Fatal(err)
	}
	if guest.OS.Family != api.OSFamilyWindows {
		t.Errorf("expected Windows, got %s", guest.OS.Family)
	}
}

func TestGuest_Linux(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.files["/etc/os-release"] = "ID=rhel\nVERSION_ID=\"9.2\"\n"
	guest, err := Guest(h)
	if err != nil {
		t.Fatal(err)
	}
	if guest.OS.Family != api.OSFamilyLinux {
		t.Errorf("expected Linux, got %s", guest.OS.Family)
	}
	if guest.OS.Distro != "rhel" {
		t.Errorf("expected rhel, got %s", guest.OS.Distro)
	}
}

// --- catGlob tests ---

func TestCatGlob(t *testing.T) {
	t.Parallel()
	h := newTestHandle()
	h.globs["/etc/sysconfig/network-scripts/ifcfg-*"] = []string{
		"/etc/sysconfig/network-scripts/ifcfg-eth0",
		"/etc/sysconfig/network-scripts/ifcfg-eth1",
	}
	h.files["/etc/sysconfig/network-scripts/ifcfg-eth0"] = "DEVICE=eth0\nBOOTPROTO=dhcp\n"
	h.files["/etc/sysconfig/network-scripts/ifcfg-eth1"] = "DEVICE=eth1\nBOOTPROTO=static\n"

	var sb strings.Builder
	err := catGlob(h, "/etc/sysconfig/network-scripts/ifcfg-*", &sb)
	if err != nil {
		t.Fatal(err)
	}
	result := sb.String()
	if !strings.Contains(result, "DEVICE=eth0") {
		t.Error("expected eth0 content")
	}
	if !strings.Contains(result, "DEVICE=eth1") {
		t.Error("expected eth1 content")
	}
}
