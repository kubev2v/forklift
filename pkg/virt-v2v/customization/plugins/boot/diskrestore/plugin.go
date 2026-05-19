// boot/windows/disk-restore — VirtIO disk online restore for legacy drivers.
//
// Applicable: Windows + VirtIoWinLegacyDrivers set.
// Output:     FileAction{Write} — 200_restore_config_legacy.ps1.
//
// Writes a PowerShell script that uses WMI (Win32_DiskDrive) and diskpart
// to bring offline VirtIO disks online. Needed only with legacy drivers that
// don't auto-online disks after conversion.
package diskrestore

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

func (p *Plugin) Name() string { return "boot/windows/disk-restore" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	return ctx.Guest.OS.IsWindows() && ctx.Config.VirtIoWinLegacyDrivers != ""
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("disk-restore plugin: nil context, guest, or config")
	}
	content, err := readScript("200_restore_config_legacy.ps1")
	if err != nil {
		return nil, fmt.Errorf("read embedded script: %w", err)
	}

	return &api.Actions{
		Files: []api.FileAction{{
			Type:      api.ActionWrite,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, "200_restore_config_legacy.ps1"),
			Content:   content,
		}},
	}, nil
}
