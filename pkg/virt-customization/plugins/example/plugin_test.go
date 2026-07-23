package example

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
)

func TestName(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	if got := p.Name(); got != "example/hello" {
		t.Errorf("Name() = %q, want %q", got, "example/hello")
	}
}

func TestApplicable_NilContext(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	if p.Applicable(nil) {
		t.Error("Applicable(nil) should return false")
	}
}

func TestApplicable_NilGuest(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	ctx := &api.Context{Config: &config.AppConfig{}}
	if p.Applicable(ctx) {
		t.Error("Applicable with nil Guest should return false")
	}
}

func TestApplicable_ReturnsFalse(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	ctx := &api.Context{
		Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
		Config: &config.AppConfig{},
	}
	if p.Applicable(ctx) {
		t.Error("example plugin Applicable should always return false")
	}
}

func TestApply_ReturnsAllActionTypes(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	ctx := &api.Context{
		Guest:  &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
		Config: &config.AppConfig{},
	}

	actions, err := p.Apply(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions.Files) == 0 {
		t.Error("expected at least one FileAction")
	}
	if len(actions.Regs) == 0 {
		t.Error("expected at least one RegAction")
	}
	if len(actions.Execs) == 0 {
		t.Error("expected at least one ExecAction")
	}

	if actions.Files[0].Type != api.ActionWrite {
		t.Errorf("FileAction type = %q, want %q", actions.Files[0].Type, api.ActionWrite)
	}
	if len(actions.Files[0].Content) == 0 {
		t.Error("FileAction content should not be empty (embedded script)")
	}
	if actions.Files[0].Permissions != "0755" {
		t.Errorf("FileAction permissions = %q, want %q", actions.Files[0].Permissions, "0755")
	}

	// ExecAction uses inline Content — no host file needed, the commit layer
	// writes it to a temp file automatically.
	if actions.Execs[0].Type != api.ActionFirstboot {
		t.Errorf("ExecAction type = %q, want %q", actions.Execs[0].Type, api.ActionFirstboot)
	}
	if len(actions.Execs[0].Content) == 0 {
		t.Error("ExecAction Content should not be empty")
	}
}

func TestApply_NilContext(t *testing.T) {
	t.Parallel()
	p := &Plugin{}
	_, err := p.Apply(nil)
	if err == nil {
		t.Error("Apply(nil) should return an error")
	}
}
