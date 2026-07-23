package commit

import (
	"errors"
	"io"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// fakeExecutor records SetStdout/SetStderr calls and returns a configurable Run error.
type fakeExecutor struct {
	runErr error
}

func (f *fakeExecutor) Run() error          { return f.runErr }
func (f *fakeExecutor) Start() error        { return nil }
func (f *fakeExecutor) Wait() error         { return nil }
func (f *fakeExecutor) SetStdout(io.Writer) {}
func (f *fakeExecutor) SetStderr(io.Writer) {}
func (f *fakeExecutor) SetStdin(io.Reader)  {}

// fakeBuilder records all calls and returns itself for chaining.
type fakeBuilder struct {
	newCmd string
	args   []string
	exec   *fakeExecutor
}

func newFakeBuilder(runErr error) *fakeBuilder {
	return &fakeBuilder{exec: &fakeExecutor{runErr: runErr}}
}

func (b *fakeBuilder) New(cmd string) utils.CommandBuilder {
	b.newCmd = cmd
	b.args = nil
	return b
}
func (b *fakeBuilder) AddArg(flag, value string) utils.CommandBuilder {
	b.args = append(b.args, flag, value)
	return b
}
func (b *fakeBuilder) AddArgs(flag string, values ...string) utils.CommandBuilder {
	for _, v := range values {
		b.args = append(b.args, flag, v)
	}
	return b
}
func (b *fakeBuilder) AddFlag(flag string) utils.CommandBuilder {
	b.args = append(b.args, flag)
	return b
}
func (b *fakeBuilder) AddPositional(value string) utils.CommandBuilder {
	b.args = append(b.args, value)
	return b
}
func (b *fakeBuilder) AddExtraArgs(values ...string) utils.CommandBuilder {
	b.args = append(b.args, values...)
	return b
}
func (b *fakeBuilder) Build() utils.CommandExecutor { return b.exec }

func (b *fakeBuilder) contains(s string) bool {
	for _, a := range b.args {
		if a == s {
			return true
		}
	}
	return false
}

func TestScripts_EmptyActions(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	err := Scripts(fb, []string{"/disk1"}, nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fb.newCmd != "" {
		t.Error("expected no command to be built for empty actions")
	}
}

func TestScripts_EmptyDisks(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.ExecAction{{Type: api.ActionFirstboot, ScriptPath: "/tmp/script.sh"}}
	err := Scripts(fb, nil, nil, actions)
	if err == nil {
		t.Fatal("expected error for empty disks")
	}
	if got := err.Error(); got != "no disks provided for virt-customize" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestScripts_UnsupportedActionType(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.ExecAction{{Type: "unknown", ScriptPath: "foo"}}
	err := Scripts(fb, []string{"/disk1"}, nil, actions)
	if err == nil {
		t.Fatal("expected error for unsupported action type")
	}
	errMsg := err.Error()
	for _, want := range []string{"unsupported", "unknown", "foo"} {
		if !containsSubstr(errMsg, want) {
			t.Errorf("error %q should contain %q", errMsg, want)
		}
	}
}

func TestScripts_ValidActions(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(nil)
	actions := []api.ExecAction{
		{Type: api.ActionFirstboot, ScriptPath: "/tmp/first.sh"},
		{Type: api.ActionRun, ScriptPath: "/tmp/run.sh"},
	}
	err := Scripts(fb, []string{"/disk1"}, []string{"all:clevis"}, actions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fb.newCmd != "virt-customize" {
		t.Errorf("expected command virt-customize, got %s", fb.newCmd)
	}
	for _, want := range []string{"-a", "/disk1", "--key", "all:clevis", "--firstboot", "/tmp/first.sh", "--run", "/tmp/run.sh"} {
		if !fb.contains(want) {
			t.Errorf("expected args to contain %q, got %v", want, fb.args)
		}
	}
}

func TestScripts_RunError(t *testing.T) {
	t.Parallel()
	fb := newFakeBuilder(errors.New("exec failed"))
	actions := []api.ExecAction{{Type: api.ActionFirstboot, ScriptPath: "/tmp/s.sh"}}
	err := Scripts(fb, []string{"/disk1"}, nil, actions)
	if err == nil {
		t.Fatal("expected error from Run")
	}
	if !containsSubstr(err.Error(), "exec failed") {
		t.Errorf("expected wrapped exec error, got %v", err)
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsIndex(s, sub))
}

func containsIndex(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
