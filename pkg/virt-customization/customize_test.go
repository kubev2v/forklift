package customization

import (
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// --- test helpers ---

type stubFS struct {
	statFn    func(string) (os.FileInfo, error)
	readDirFn func(string) ([]os.DirEntry, error)
}

func (f *stubFS) Symlink(_, _ string) error                         { return nil }
func (f *stubFS) WriteFile(_ string, _ []byte, _ os.FileMode) error { return nil }
func (f *stubFS) Stat(name string) (os.FileInfo, error) {
	if f.statFn != nil {
		return f.statFn(name)
	}
	return nil, os.ErrNotExist
}
func (f *stubFS) ReadDir(name string) ([]os.DirEntry, error) {
	if f.readDirFn != nil {
		return f.readDirFn(name)
	}
	return nil, nil
}

type stubPlugin struct {
	name     string
	actions  *api.Actions
	applyErr error
}

func (p *stubPlugin) Name() string                               { return p.name }
func (p *stubPlugin) Applicable(_ *api.Context) bool             { return true }
func (p *stubPlugin) Apply(_ *api.Context) (*api.Actions, error) { return p.actions, p.applyErr }

// stubHandle implements api.GuestHandle for tests.
type stubHandle struct{}

func (s *stubHandle) IsDir(string) (bool, error)          { return false, nil }
func (s *stubHandle) IsFile(string) (bool, error)         { return false, nil }
func (s *stubHandle) Cat(string) (string, error)          { return "", nil }
func (s *stubHandle) GlobExpand(string) ([]string, error) { return nil, nil }
func (s *stubHandle) Ls(string) ([]string, error)         { return nil, nil }
func (s *stubHandle) ReadFile(string) ([]byte, error)     { return nil, nil }
func (s *stubHandle) MkdirP(string) error                 { return nil }
func (s *stubHandle) Upload(string, string) error         { return nil }
func (s *stubHandle) Write(string, []byte) error          { return nil }
func (s *stubHandle) Chmod(int, string) error             { return nil }
func (s *stubHandle) Shutdown() error                     { return nil }
func (s *stubHandle) Close()                              {}

var _ api.GuestHandle = (*stubHandle)(nil)

func noopOpenHandle(_ []string, _ []string, _ string) (api.GuestHandle, error) {
	return &stubHandle{}, nil
}

func noopProbe(_ api.GuestHandle) (*api.GuestInfo, error) {
	return &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}}, nil
}

func noopCommitFiles(_ api.GuestHandle, _ []api.FileAction) error {
	return nil
}

func noopCommitRegistry(_ utils.CommandBuilder, _ []string, _ []string, _ []api.RegAction) error {
	return nil
}

func noopCommitScripts(_ utils.CommandBuilder, _ []string, _ []string, _ []api.ExecAction) error {
	return nil
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- luksKeys tests ---

func TestLuksKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *config.AppConfig
		fs       utils.FileSystem
		wantKeys []string
		wantErr  string
	}{
		{
			name:     "NbdeClevis true returns clevis key",
			cfg:      &config.AppConfig{NbdeClevis: true},
			fs:       &stubFS{},
			wantKeys: []string{"all:clevis"},
		},
		{
			name: "empty Luksdir returns nil",
			cfg:  &config.AppConfig{},
			fs:   &stubFS{},
		},
		{
			name: "Stat returns os.ErrNotExist returns nil",
			cfg:  &config.AppConfig{Luksdir: "/etc/luks"},
			fs: &stubFS{
				statFn: func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
			},
		},
		{
			name: "Stat returns other error wraps message",
			cfg:  &config.AppConfig{Luksdir: "/etc/luks"},
			fs: &stubFS{
				statFn: func(string) (os.FileInfo, error) {
					return nil, errors.New("permission denied")
				},
			},
			wantErr: "error accessing LUKS directory",
		},
		{
			name: "GetFilesInPath (ReadDir) error wraps message",
			cfg:  &config.AppConfig{Luksdir: "/etc/luks"},
			fs: &stubFS{
				statFn: func(string) (os.FileInfo, error) {
					return utils.MockFileInfo{}, nil
				},
				readDirFn: func(string) ([]os.DirEntry, error) {
					return nil, errors.New("i/o timeout")
				},
			},
			wantErr: "error reading LUKS key files",
		},
		{
			name: "successful key list",
			cfg:  &config.AppConfig{Luksdir: "/etc/luks"},
			fs: &stubFS{
				statFn: func(string) (os.FileInfo, error) {
					return utils.MockFileInfo{}, nil
				},
				readDirFn: func(string) ([]os.DirEntry, error) {
					return utils.ConvertMockDirEntryToOs([]utils.MockDirEntry{
						{FileName: "key1"},
						{FileName: "key2"},
					}), nil
				},
			},
			wantKeys: []string{"all:file:/etc/luks/key1", "all:file:/etc/luks/key2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			keys, err := luksKeys(tc.fs, tc.cfg)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slicesEqual(keys, tc.wantKeys) {
				t.Errorf("keys = %v, want %v", keys, tc.wantKeys)
			}
		})
	}
}

// --- Run tests ---

func TestRun(t *testing.T) {
	t.Parallel()

	noopFS := &stubFS{}

	t.Run("missing Config returns error", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{Disks: []string{"/dev/sda"}, OpenHandle: noopOpenHandle})
		if err == nil || !strings.Contains(err.Error(), "Run: missing Config") {
			t.Fatalf("expected 'Run: missing Config', got: %v", err)
		}
	})

	t.Run("empty Disks returns error", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{Config: &config.AppConfig{}, OpenHandle: noopOpenHandle})
		if err == nil || !strings.Contains(err.Error(), "Run: no Disks provided") {
			t.Fatalf("expected 'Run: no Disks provided', got: %v", err)
		}
	})

	t.Run("missing OpenHandle returns error", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{Config: &config.AppConfig{}, Disks: []string{"/dev/sda"}})
		if err == nil || !strings.Contains(err.Error(), "Run: missing OpenHandle") {
			t.Fatalf("expected 'Run: missing OpenHandle', got: %v", err)
		}
	})

	t.Run("OpenHandle failure propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config: &config.AppConfig{},
			Disks:  []string{"/dev/sda"},
			OpenHandle: func([]string, []string, string) (api.GuestHandle, error) {
				return nil, errors.New("appliance failed")
			},
			FileSystem: noopFS,
		})
		if err == nil || !strings.Contains(err.Error(), "opening guest handle:") {
			t.Fatalf("expected 'opening guest handle:' wrapper, got: %v", err)
		}
	})

	t.Run("probe failure propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: func(_ api.GuestHandle) (*api.GuestInfo, error) {
				return nil, errors.New("connection refused")
			},
		})
		if err == nil || !strings.Contains(err.Error(), "guest probe:") {
			t.Fatalf("expected 'guest probe:' wrapper, got: %v", err)
		}
		if !strings.Contains(err.Error(), "connection refused") {
			t.Fatalf("expected original error preserved, got: %v", err)
		}
	})

	t.Run("plugin Apply error wrapped with plugin name", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{name: "test/exploder", applyErr: errors.New("boom")},
			},
			commitFiles:    noopCommitFiles,
			commitRegistry: noopCommitRegistry,
			commitScripts:  noopCommitScripts,
		})
		if err == nil || !strings.Contains(err.Error(), "plugin test/exploder:") {
			t.Fatalf("expected 'plugin test/exploder:' wrapper, got: %v", err)
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected original error preserved, got: %v", err)
		}
	})

	t.Run("file actions trigger commitFiles", func(t *testing.T) {
		t.Parallel()
		var filesCalled atomic.Bool
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/writer",
					actions: &api.Actions{
						Files: []api.FileAction{{Type: api.ActionWrite, GuestPath: "/etc/test", Content: []byte("data")}},
					},
				},
			},
			commitFiles: func(_ api.GuestHandle, files []api.FileAction) error {
				filesCalled.Store(true)
				if len(files) != 1 {
					t.Errorf("expected 1 file action, got %d", len(files))
				}
				return nil
			},
			commitRegistry: noopCommitRegistry,
			commitScripts:  noopCommitScripts,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filesCalled.Load() {
			t.Error("expected commitFiles to be called")
		}
	})

	t.Run("commitFiles not called when no file actions", func(t *testing.T) {
		t.Parallel()
		var filesCalled atomic.Bool
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins:    []api.Plugin{&stubPlugin{name: "test/noop", actions: &api.Actions{}}},
			commitFiles: func(_ api.GuestHandle, _ []api.FileAction) error {
				filesCalled.Store(true)
				return nil
			},
			commitRegistry: noopCommitRegistry,
			commitScripts:  noopCommitScripts,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filesCalled.Load() {
			t.Error("commitFiles should not be called when there are no file actions")
		}
	})

	t.Run("exec actions trigger commitScripts", func(t *testing.T) {
		t.Parallel()
		var scriptsCalled atomic.Bool
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/runner",
					actions: &api.Actions{
						Execs: []api.ExecAction{{Type: api.ActionFirstboot, ScriptPath: "/tmp/init.sh"}},
					},
				},
			},
			commitFiles:    noopCommitFiles,
			commitRegistry: noopCommitRegistry,
			commitScripts: func(_ utils.CommandBuilder, _ []string, _ []string, execs []api.ExecAction) error {
				scriptsCalled.Store(true)
				if len(execs) != 1 {
					t.Errorf("expected 1 exec action, got %d", len(execs))
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !scriptsCalled.Load() {
			t.Error("expected commitScripts to be called")
		}
	})

	t.Run("commitFiles error propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/writer",
					actions: &api.Actions{
						Files: []api.FileAction{{Type: api.ActionWrite, GuestPath: "/etc/test"}},
					},
				},
			},
			commitFiles: func(_ api.GuestHandle, _ []api.FileAction) error {
				return errors.New("disk full")
			},
			commitRegistry: noopCommitRegistry,
			commitScripts:  noopCommitScripts,
		})
		if err == nil || !strings.Contains(err.Error(), "commit files:") {
			t.Fatalf("expected 'commit files:' wrapper, got: %v", err)
		}
	})

	t.Run("commitScripts error propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/runner",
					actions: &api.Actions{
						Execs: []api.ExecAction{{Type: api.ActionRun, ScriptPath: "/tmp/fix.sh"}},
					},
				},
			},
			commitFiles:    noopCommitFiles,
			commitRegistry: noopCommitRegistry,
			commitScripts: func(_ utils.CommandBuilder, _ []string, _ []string, _ []api.ExecAction) error {
				return errors.New("virt-customize segfault")
			},
		})
		if err == nil || !strings.Contains(err.Error(), "virt-customize exec:") {
			t.Fatalf("expected 'virt-customize exec:' wrapper, got: %v", err)
		}
	})

	t.Run("LUKS key resolution error propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{Luksdir: "/etc/luks"},
			Disks:      []string{"/dev/sda"},
			OpenHandle: noopOpenHandle,
			FileSystem: &stubFS{
				statFn: func(string) (os.FileInfo, error) {
					return nil, errors.New("io error")
				},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "resolving LUKS keys:") {
			t.Fatalf("expected 'resolving LUKS keys:' wrapper, got: %v", err)
		}
	})

	t.Run("reg actions trigger commitRegistry", func(t *testing.T) {
		t.Parallel()
		var registryCalled atomic.Bool
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/regwriter",
					actions: &api.Actions{
						Regs: []api.RegAction{{Content: []byte("Windows Registry Editor Version 5.00\n")}},
					},
				},
			},
			commitFiles: noopCommitFiles,
			commitRegistry: func(_ utils.CommandBuilder, _ []string, _ []string, regs []api.RegAction) error {
				registryCalled.Store(true)
				if len(regs) != 1 {
					t.Errorf("expected 1 reg action, got %d", len(regs))
				}
				return nil
			},
			commitScripts: noopCommitScripts,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !registryCalled.Load() {
			t.Error("expected commitRegistry to be called")
		}
	})

	t.Run("commitRegistry not called when no reg actions", func(t *testing.T) {
		t.Parallel()
		var registryCalled atomic.Bool
		err := Run(Options{
			Config:      &config.AppConfig{},
			Disks:       []string{"/dev/sda"},
			FileSystem:  noopFS,
			OpenHandle:  noopOpenHandle,
			probeGuest:  noopProbe,
			plugins:     []api.Plugin{&stubPlugin{name: "test/noop", actions: &api.Actions{}}},
			commitFiles: noopCommitFiles,
			commitRegistry: func(_ utils.CommandBuilder, _ []string, _ []string, _ []api.RegAction) error {
				registryCalled.Store(true)
				return nil
			},
			commitScripts: noopCommitScripts,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if registryCalled.Load() {
			t.Error("commitRegistry should not be called when there are no reg actions")
		}
	})

	t.Run("commitRegistry error propagated", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:     &config.AppConfig{},
			Disks:      []string{"/dev/sda"},
			FileSystem: noopFS,
			OpenHandle: noopOpenHandle,
			probeGuest: noopProbe,
			plugins: []api.Plugin{
				&stubPlugin{
					name: "test/regwriter",
					actions: &api.Actions{
						Regs: []api.RegAction{{Content: []byte("Windows Registry Editor Version 5.00\n")}},
					},
				},
			},
			commitFiles: noopCommitFiles,
			commitRegistry: func(_ utils.CommandBuilder, _ []string, _ []string, _ []api.RegAction) error {
				return errors.New("registry hive corrupted")
			},
			commitScripts: noopCommitScripts,
		})
		if err == nil || !strings.Contains(err.Error(), "virt-win-reg merge:") {
			t.Fatalf("expected 'virt-win-reg merge:' wrapper, got: %v", err)
		}
	})

	t.Run("happy path with no plugins produces no error", func(t *testing.T) {
		t.Parallel()
		err := Run(Options{
			Config:         &config.AppConfig{},
			Disks:          []string{"/dev/sda"},
			FileSystem:     noopFS,
			OpenHandle:     noopOpenHandle,
			probeGuest:     noopProbe,
			plugins:        []api.Plugin{},
			commitFiles:    noopCommitFiles,
			commitRegistry: noopCommitRegistry,
			commitScripts:  noopCommitScripts,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
