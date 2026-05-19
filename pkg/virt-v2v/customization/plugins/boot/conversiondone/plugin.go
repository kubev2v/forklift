// boot/windows/conversion-done — COM1 signal after post-conversion reboot.
//
// Applicable: Windows + WaitForGuestReboot flag.
// Output:     FileAction{Write} — 990_signal_conversion_done.ps1.
//
// Writes a PowerShell script that opens COM1 (serial port) and writes
// CONVERSION_DONE, signaling the MTV controller that the guest has
// completed its post-conversion reboot cycle. Numbered 990_ to run last.
package conversiondone

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

func (p *Plugin) Name() string { return "boot/windows/conversion-done" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	return ctx.Guest.OS.IsWindows() && ctx.Config.WaitForGuestReboot
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("conversion-done plugin: nil context, guest, or config")
	}
	content, err := readScript("990_signal_conversion_done.ps1")
	if err != nil {
		return nil, fmt.Errorf("read embedded script: %w", err)
	}

	return &api.Actions{
		Files: []api.FileAction{{
			Type:      api.ActionWrite,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, "990_signal_conversion_done.ps1"),
			Content:   content,
		}},
	}, nil
}
