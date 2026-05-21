// network/netsh — Windows static IP configuration via WMI and netsh (legacy).
//
// Applicable: Windows + StaticIPs set + VirtIoWinLegacyDrivers set +
//
//	NOT WindowsRegistryNetworkConfig.
//
// Output:     FileAction{Write} — 100_network_config.ps1, 120_remove_duplicate_routes.ps1.
//
// Legacy path for older Windows or VirtIO drivers. Uses WMI (Win32_NetworkAdapter)
// and netsh to configure static IPs. Mutually exclusive with registry — the
// registry plugin takes priority when both flags are set.
package netsh

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"text/template"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
)

//go:embed scripts
var scriptFS embed.FS

// readScript returns the content of the named embedded script file.
func readScript(name string) ([]byte, error) {
	return scriptFS.ReadFile("scripts/" + name)
}

type Plugin struct{}

func (p *Plugin) Name() string { return "network/netsh" }

func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	if !ctx.Guest.OS.IsWindows() {
		return false
	}
	if ctx.Config.StaticIPs == "" {
		return false
	}
	if ctx.Config.WindowsRegistryNetworkConfig {
		return false
	}
	return ctx.Config.VirtIoWinLegacyDrivers != ""
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("netsh plugin: nil context, guest, or config")
	}
	var actions api.Actions

	rendered, err := renderTemplate("100_network_config.ps1.tmpl", ctx.Config.StaticIPs)
	if err != nil {
		return nil, fmt.Errorf("inject netsh static IP template: %w", err)
	}
	actions.Files = append(actions.Files, api.FileAction{
		Type:      api.ActionWrite,
		GuestPath: path.Join(api.WinFirstbootScriptsPath, "100_network_config.ps1"),
		Content:   rendered,
	})

	removeDuplicates, err := readScript("120_remove_duplicate_routes.ps1")
	if err != nil {
		return nil, fmt.Errorf("read embedded script: %w", err)
	}
	actions.Files = append(actions.Files, api.FileAction{
		Type:      api.ActionWrite,
		GuestPath: path.Join(api.WinFirstbootScriptsPath, "120_remove_duplicate_routes.ps1"),
		Content:   removeDuplicates,
	})

	return &actions, nil
}

// renderTemplate parses the named embedded Go template and renders it with staticIPs.
func renderTemplate(tmplName, staticIPs string) ([]byte, error) {
	tmplContent, err := readScript(tmplName)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", tmplName, err)
	}
	tmpl, err := template.New("netConfig").Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	data := struct{ InputString string }{InputString: staticIPs}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	return buf.Bytes(), nil
}
