// drivers/vmware — VMware Tools driver and service removal for Windows.
//
// Applicable: Windows + VsphereVmwareDriverRemoval flag + source is vSphere.
// Output:     FileAction{Write} — 5 .bat scripts (100–104).
//
// Writes batch scripts that disable, stop, and remove VMware Tools services,
// drivers, and registry entries. Only applies to vSphere-sourced Windows VMs.
package vmware

import (
	"embed"
	"fmt"
	"path"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

//go:embed scripts
var scriptFS embed.FS

// readScript returns the content of the named embedded script file.
func readScript(name string) ([]byte, error) {
	return scriptFS.ReadFile("scripts/" + name)
}

// driverRemovalSteps maps execution-order prefix to source script filename.
// The firstboot service runs .bat files in alphabetical order.
var driverRemovalScripts = []string{
	"100_disable_services.bat",
	"101_remove_services.bat",
	"102_remove_drivers.bat",
	"103_query_registry.bat",
	"104_remove_registry.bat",
}

type Plugin struct{}

func (p *Plugin) Name() string { return "drivers/vmware" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	if !ctx.Guest.OS.IsWindows() {
		return false
	}
	return ctx.Config.VsphereVmwareDriverRemoval && ctx.Config.Source == config.VSPHERE
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	var actions api.Actions
	for _, name := range driverRemovalScripts {
		content, err := readScript(name)
		if err != nil {
			return nil, fmt.Errorf("read embedded script %s: %w", name, err)
		}
		actions.Files = append(actions.Files, api.FileAction{
			Type:      api.ActionWrite,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, name),
			Content:   content,
		})
	}
	return &actions, nil
}
