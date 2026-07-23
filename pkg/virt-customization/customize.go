package customization

import (
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-customization/commit"
	"github.com/kubev2v/forklift/pkg/virt-customization/guesthandle"
	"github.com/kubev2v/forklift/pkg/virt-customization/probe"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Options configures a post-conversion run.
type Options struct {
	Config *config.AppConfig
	Disks  []string

	// OpenHandle creates the libguestfs session for probe + file
	// operations (Phases 1+2). Required in production.
	OpenHandle guesthandle.HandleFactory

	// CommandBuilder is used only for CLI-based phases (registry + scripts).
	// Optional — defaults to a real CommandBuilder.
	CommandBuilder utils.CommandBuilder
	FileSystem     utils.FileSystem

	// Unexported test hooks — nil means use real implementation.
	probeGuest     func(api.GuestHandle) (*api.GuestInfo, error)
	plugins        []api.Plugin
	commitFiles    func(api.GuestHandle, []api.FileAction) error
	commitRegistry func(utils.CommandBuilder, []string, []string, []api.RegAction) error
	commitScripts  func(utils.CommandBuilder, []string, []string, []api.ExecAction) error
}

// Run is the self-contained entry point for post-conversion processing.
// It operates in four phases:
//  1. Probe guest via GuestHandle (detect OS, stacks, interfaces)
//  2. Apply file operations via GuestHandle (uploads, writes, mkdir, chmod)
//     Then the GuestHandle session is shut down so CLI tools get exclusive access.
//  3. Merge Windows Registry entries via virt-win-reg --merge (if any)
//  4. Execute scripts via virt-customize (only --firstboot/--run if needed)
//
// LUKS keys (when configured) are passed to CLI phases so that encrypted
// volumes can be unlocked.
func Run(opts Options) error {
	if opts.Config == nil {
		return fmt.Errorf("Run: missing Config")
	}
	if len(opts.Disks) == 0 {
		return fmt.Errorf("Run: no Disks provided")
	}
	if opts.OpenHandle == nil {
		return fmt.Errorf("Run: missing OpenHandle factory")
	}

	applyDefaults(&opts)

	// Resolve LUKS keys — needed for the GuestHandle and CLI phases.
	keys, err := luksKeys(opts.FileSystem, opts.Config)
	if err != nil {
		return fmt.Errorf("resolving LUKS keys: %w", err)
	}

	// Open the GuestHandle for Phases 1+2.
	g, err := opts.OpenHandle(opts.Disks, keys, opts.Config.RootDisk)
	if err != nil {
		return fmt.Errorf("opening guest handle: %w", err)
	}

	// Phase 1: Probe guest via GuestHandle
	doProbe := probe.Guest
	if opts.probeGuest != nil {
		doProbe = opts.probeGuest
	}
	guest, err := doProbe(g)
	if err != nil {
		g.Close()
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
	var allRegs []api.RegAction
	var allExecs []api.ExecAction

	for _, p := range plugins {
		fmt.Printf("Collecting actions from plugin: %s\n", p.Name())
		actions, err := p.Apply(ctx)
		if err != nil {
			g.Close()
			return fmt.Errorf("plugin %s: %w", p.Name(), err)
		}
		if actions == nil {
			continue
		}
		allFiles = append(allFiles, actions.Files...)
		allRegs = append(allRegs, actions.Regs...)
		allExecs = append(allExecs, actions.Execs...)
	}

	// Phase 2: Apply file operations via GuestHandle
	doFiles := commit.Files
	if opts.commitFiles != nil {
		doFiles = opts.commitFiles
	}
	if len(allFiles) > 0 {
		if err := doFiles(g, allFiles); err != nil {
			g.Close()
			return fmt.Errorf("commit files: %w", err)
		}
	}

	// Shut down the GuestHandle before CLI phases to release disk locks.
	if err := g.Shutdown(); err != nil {
		fmt.Printf("warning: GuestHandle shutdown: %v\n", err)
	}
	g.Close()

	// Phase 3: Merge Windows Registry entries via virt-win-reg (CLI, only if needed)
	if len(allRegs) > 0 {
		doRegistry := commit.Registry
		if opts.commitRegistry != nil {
			doRegistry = opts.commitRegistry
		}
		if err := doRegistry(opts.CommandBuilder, opts.Disks, keys, allRegs); err != nil {
			return fmt.Errorf("virt-win-reg merge: %w", err)
		}
	}

	// Phase 4: Execute scripts via virt-customize (CLI, only if needed)
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
// virt-customize and virt-win-reg so encrypted volumes can be accessed.
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
