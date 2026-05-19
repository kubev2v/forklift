package customization

import (
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/commit"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/probe"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Options configures a post-conversion run.
type Options struct {
	Config *config.AppConfig
	Disks  []string
	// Optional overrides for testing; nil means use real implementations.
	CommandBuilder utils.CommandBuilder
	FileSystem     utils.FileSystem

	// Unexported test hooks — nil means use real implementation.
	// Production callers never set these.
	probeGuest    func(utils.CommandBuilder, []string, []string, string) (*api.GuestInfo, error)
	plugins       []api.Plugin
	commitFiles   func(utils.CommandBuilder, []string, []string, string, []api.FileAction) error
	commitScripts func(utils.CommandBuilder, []string, []string, []api.ExecAction) error
}

// Run is the self-contained entry point for post-conversion processing.
// It operates in three phases:
//  1. Probe guest via guestfish --ro (detect OS, stacks, interfaces)
//  2. Apply file operations via guestfish --rw (uploads, writes)
//  3. Execute scripts via virt-customize (only --firstboot/--run if needed)
//
// LUKS keys (when configured) are passed to all phases so that encrypted
// volumes can be unlocked for inspection, file writes, and script execution.
func Run(opts Options) error {
	if opts.Config == nil {
		return fmt.Errorf("Run: missing Config")
	}
	if len(opts.Disks) == 0 {
		return fmt.Errorf("Run: no Disks provided")
	}

	applyDefaults(&opts)

	// Resolve LUKS keys early -- all phases need them to access encrypted disks.
	keys, err := luksKeys(opts.FileSystem, opts.Config)
	if err != nil {
		return fmt.Errorf("resolving LUKS keys: %w", err)
	}

	// Phase 1: Probe guest
	doProbe := probe.Guest
	if opts.probeGuest != nil {
		doProbe = opts.probeGuest
	}
	guest, err := doProbe(opts.CommandBuilder, opts.Disks, keys, opts.Config.RootDisk)
	if err != nil {
		return fmt.Errorf("guest probe: %w", err)
	}

	ctx := &api.Context{
		Guest:      guest,
		Config:     opts.Config,
		Disks:      opts.Disks,
		FileSystem: opts.FileSystem,
	}

	// Resolve and collect all actions
	plugins := opts.plugins
	if plugins == nil {
		plugins = Resolve(ctx)
	}
	var allFiles []api.FileAction
	var allExecs []api.ExecAction

	for _, p := range plugins {
		fmt.Printf("Collecting actions from plugin: %s\n", p.Name())
		actions, err := p.Apply(ctx)
		if err != nil {
			return fmt.Errorf("plugin %s: %w", p.Name(), err)
		}
		if actions == nil {
			continue
		}
		allFiles = append(allFiles, actions.Files...)
		allExecs = append(allExecs, actions.Execs...)
	}

	// Phase 2: Apply file operations via guestfish --rw
	doFiles := commit.Files
	if opts.commitFiles != nil {
		doFiles = opts.commitFiles
	}
	if len(allFiles) > 0 {
		if err := doFiles(opts.CommandBuilder, opts.Disks, keys, opts.Config.RootDisk, allFiles); err != nil {
			return fmt.Errorf("guestfish apply: %w", err)
		}
	}

	// Phase 3: Execute scripts via virt-customize (only if needed)
	if len(allExecs) > 0 {
		doScripts := commit.Scripts
		if opts.commitScripts != nil {
			doScripts = opts.commitScripts
		}
		if err := doScripts(opts.CommandBuilder, opts.Disks, keys, allExecs); err != nil {
			return fmt.Errorf("virt-customize exec: %w", err)
		}
	}

	return nil
}

// luksKeys computes LUKS key arguments from config, supporting both
// key files and centralized server (NBDE/Clevis). These are passed to
// guestfish and virt-customize so encrypted volumes can be accessed.
func luksKeys(fs utils.FileSystem, cfg *config.AppConfig) ([]string, error) {
	if cfg.NbdeClevis {
		return []string{"all:clevis"}, nil
	}
	if cfg.Luksdir == "" {
		return nil, nil
	}
	if _, err := fs.Stat(cfg.Luksdir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error accessing LUKS directory: %w", err)
	}
	files, err := utils.GetFilesInPath(fs, cfg.Luksdir)
	if err != nil {
		return nil, fmt.Errorf("error reading LUKS key files: %w", err)
	}
	var keys []string
	for _, file := range files {
		keys = append(keys, fmt.Sprintf("all:file:%s", file))
	}
	return keys, nil
}

func applyDefaults(opts *Options) {
	if opts.CommandBuilder == nil {
		opts.CommandBuilder = &utils.CommandBuilderImpl{}
	}
	if opts.FileSystem == nil {
		opts.FileSystem = &utils.FileSystemImpl{}
	}
}
