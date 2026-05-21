// boot/windows/firstboot-runner — .bat orchestrator for Windows firstboot scripts.
//
// Applicable: Windows (always).
// Output:     FileAction{Write} — 900_firstboot_init.bat or 900_firstboot_init_legacy.bat.
//
// Writes the .bat orchestrator that iterates over all .ps1 scripts in the
// firstboot directory and executes them in alphabetical order. The legacy
// variant sets the PowerShell execution policy to Unrestricted via registry
// before running scripts (needed for older Windows without -ExecutionPolicy
// Bypass support).
package firstboot

import (
	"embed"
	"fmt"
	"path"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

//go:embed scripts
var scriptFS embed.FS

// readScript returns the content of the named embedded script file.
func readScript(name string) ([]byte, error) {
	return scriptFS.ReadFile("scripts/" + name)
}

type Plugin struct{}

func (p *Plugin) Name() string { return "boot/windows/firstboot-runner" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil {
		return false
	}
	return ctx.Guest.OS.IsWindows()
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Config == nil || ctx.Guest == nil {
		return nil, fmt.Errorf("firstboot plugin: nil context, config, or guest")
	}
	script := "900_firstboot_init.bat"
	if ctx.Config.VirtIoWinLegacyDrivers != "" {
		script = "900_firstboot_init_legacy.bat"
	}

	content, err := readScript(script)
	if err != nil {
		return nil, fmt.Errorf("read embedded script %s: %w", script, err)
	}

	return &api.Actions{
		Files: []api.FileAction{{
			Type:      api.ActionWrite,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, script),
			Content:   content,
		}},
	}, nil
}
