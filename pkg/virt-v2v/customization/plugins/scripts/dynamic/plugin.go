// scripts/dynamic — User-supplied custom scripts from a ConfigMap mount.
//
// Applicable: DynamicScriptsDir exists on the host filesystem.
// Output:     Linux: ExecAction{Run/Firstboot} — Windows: FileAction{Upload}.
//
// Scans the ConfigMap-mounted directory for user-supplied scripts matching
// naming conventions:
//   - Linux:   NNN_linux_(run|firstboot)_name.sh  → --run or --firstboot
//   - Windows: NNN_win_firstboot_name.ps1         → uploaded to guest firstboot dir
//
// Directories and non-matching files are ignored.
package dynamic

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

const (
	windowsDynamicRegex = `^([0-9]+_win_firstboot(([\w\-]*)\.ps1))$`
	linuxDynamicRegex   = `^([0-9]+_linux_(run|firstboot)(([\w\-]*)\.sh))$`
)

type Plugin struct{}

func (p *Plugin) Name() string { return "scripts/dynamic" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Config == nil || ctx.FileSystem == nil {
		return false
	}
	if ctx.Config.DynamicScriptsDir == "" {
		return false
	}
	_, err := ctx.FileSystem.Stat(ctx.Config.DynamicScriptsDir)
	return err == nil
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil {
		return &api.Actions{}, nil
	}
	switch ctx.Guest.OS.Family {
	case api.OSFamilyWindows:
		return p.addWindowsDynamic(ctx)
	case api.OSFamilyLinux:
		return p.addLinuxDynamic(ctx)
	default:
		return &api.Actions{}, nil
	}
}

// addWindowsDynamic scans DynamicScriptsDir for Windows scripts and returns upload actions.
func (p *Plugin) addWindowsDynamic(ctx *api.Context) (*api.Actions, error) {
	scripts, err := getScriptsWithRegex(ctx, ctx.Config.DynamicScriptsDir, windowsDynamicRegex)
	if err != nil {
		return nil, err
	}
	var actions api.Actions
	for _, script := range scripts {
		fmt.Printf("Adding windows dynamic script '%s'\n", script.path)
		actions.Files = append(actions.Files, api.FileAction{
			Type:      api.ActionUpload,
			LocalPath: script.path,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, filepath.Base(script.path)),
		})
	}
	return &actions, nil
}

// addLinuxDynamic scans DynamicScriptsDir for Linux scripts and returns run/firstboot exec actions.
func (p *Plugin) addLinuxDynamic(ctx *api.Context) (*api.Actions, error) {
	scripts, err := getScriptsWithRegex(ctx, ctx.Config.DynamicScriptsDir, linuxDynamicRegex)
	if err != nil {
		return nil, err
	}
	var actions api.Actions
	for _, script := range scripts {
		fmt.Printf("Adding linux dynamic script '%s'\n", script.path)
		action := script.groups[2]
		switch action {
		case "run":
			actions.Execs = append(actions.Execs, api.ExecAction{
				Type: api.ActionRun, Value: script.path,
			})
		case "firstboot":
			actions.Execs = append(actions.Execs, api.ExecAction{
				Type: api.ActionFirstboot, Value: script.path,
			})
		default:
			return nil, fmt.Errorf("invalid action '%s' from script '%s': expected 'run' or 'firstboot'", action, script.path)
		}
	}
	return &actions, nil
}

type scriptMatch struct {
	path   string
	groups []string
}

// getScriptsWithRegex reads directory and returns files matching pattern, sorted by name.
func getScriptsWithRegex(ctx *api.Context, directory, pattern string) ([]scriptMatch, error) {
	files, err := ctx.FileSystem.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read scripts directory %s: %w", directory, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	r := regexp.MustCompile(pattern)
	var scripts []scriptMatch
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		groups := r.FindStringSubmatch(file.Name())
		if groups != nil {
			scripts = append(scripts, scriptMatch{
				path:   filepath.Join(directory, file.Name()),
				groups: groups,
			})
		}
	}
	return scripts, nil
}
