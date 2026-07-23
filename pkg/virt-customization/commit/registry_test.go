package commit

import (
	"errors"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

func TestRegistry_EmptyActions(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	err := Registry(fb, []string{"/disk1"}, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fb.newCmd != "" {
		t.Error("expected no command to be built for empty actions")
	}
}

func TestRegistry_EmptyDisks(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.RegAction{{Content: []byte("Windows Registry Editor Version 5.00\n")}}
	err := Registry(fb, nil, nil, actions)
	if err == nil {
		t.Fatal("expected error for empty disks")
	}
	if got := err.Error(); got != "no disks provided for virt-win-reg" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRegistry_ValidActions(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.RegAction{
		{Content: []byte("Windows Registry Editor Version 5.00\n\n[HKEY_LOCAL_MACHINE\\SOFTWARE\\Test]\n\"Key\"=\"Value\"\n")},
	}
	err := Registry(fb, []string{"/disk1"}, []string{"all:clevis"}, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fb.newCmd != "virt-win-reg" {
		t.Errorf("expected command virt-win-reg, got %s", fb.newCmd)
	}
	for _, want := range []string{"--merge", "-a", "/disk1", "--key", "all:clevis"} {
		if !fb.contains(want) {
			t.Errorf("expected args to contain %q, got %v", want, fb.args)
		}
	}
}

func TestRegistry_MultipleDisksAndKeys(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.RegAction{
		{Content: []byte("Windows Registry Editor Version 5.00\n")},
	}
	err := Registry(fb, []string{"/disk1", "/disk2"}, []string{"all:file:/k1", "all:file:/k2"}, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"/disk1", "/disk2", "all:file:/k1", "all:file:/k2"} {
		if !fb.contains(want) {
			t.Errorf("expected args to contain %q, got %v", want, fb.args)
		}
	}
}

func TestRegistry_MultipleActions(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.RegAction{
		{Content: []byte("Windows Registry Editor Version 5.00\n\n[HKEY_LOCAL_MACHINE\\SOFTWARE\\A]\n")},
		{Content: []byte("Windows Registry Editor Version 5.00\n\n[HKEY_LOCAL_MACHINE\\SOFTWARE\\B]\n")},
	}
	err := Registry(fb, []string{"/disk1"}, nil, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	positionalCount := 0
	for _, a := range fb.args {
		if len(a) > 0 && a[0] != '-' && a != "/disk1" {
			positionalCount++
		}
	}
	if positionalCount != 2 {
		t.Errorf("expected 2 positional .reg file args, got %d (args: %v)", positionalCount, fb.args)
	}
}

func TestRegistry_RunError(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(errors.New("merge failed"))
	actions := []api.RegAction{{Content: []byte("Windows Registry Editor Version 5.00\n")}}
	err := Registry(fb, []string{"/disk1"}, nil, actions)
	if err == nil {
		t.Fatal("expected error from Run")
	}
	if !containsSubstr(err.Error(), "merge failed") {
		t.Errorf("expected wrapped merge error, got %v", err)
	}
}
