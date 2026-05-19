// network/registry — Windows static IP configuration via direct registry writes.
//
// Applicable: Windows + StaticIPs set + WindowsRegistryNetworkConfig flag.
// Output:     FileAction{Write} — 100_network_config.ps1, 120_remove_duplicate_routes.ps1,
//
//	optionally 110_complementary_ips.ps1.
//
// Renders PowerShell templates that configure static IPs via direct registry
// writes to HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces.
// Uses Get-NetAdapter to find adapters by MAC. Does not depend on DHCP Client
// or RPC services. The 110_complementary_ips.ps1 script is added only when
// MultipleIpsPerNicName is set (multiple IPs per NIC).
package registry

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"text/template"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/network/registry/staticip"
)

//go:embed scripts
var scriptFS embed.FS

// readScript returns the content of the named embedded script file.
func readScript(name string) ([]byte, error) {
	return scriptFS.ReadFile("scripts/" + name)
}

type Plugin struct{}

func (p *Plugin) Name() string { return "network/registry" }

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
	return ctx.Config.WindowsRegistryNetworkConfig
}

func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("registry plugin: nil context, guest, or config")
	}
	var actions api.Actions

	rendered, err := injectStaticIPTemplate(ctx.Config.StaticIPs)
	if err != nil {
		return nil, fmt.Errorf("inject registry static IP template: %w", err)
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

	if ctx.Config.MultipleIpsPerNicName != "" {
		complementary, err := injectComplementaryIPTemplate(ctx.Config.StaticIPs)
		if err != nil {
			return nil, fmt.Errorf("inject complementary IP template: %w", err)
		}
		actions.Files = append(actions.Files, api.FileAction{
			Type:      api.ActionWrite,
			GuestPath: path.Join(api.WinFirstbootScriptsPath, "110_complementary_ips.ps1"),
			Content:   complementary,
		})
	}

	return &actions, nil
}

// injectStaticIPTemplate renders 100_network_config.ps1.tmpl with the given staticIPs string.
func injectStaticIPTemplate(staticIPs string) ([]byte, error) {
	tmplContent, err := readScript("100_network_config.ps1.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
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

// injectComplementaryIPTemplate renders 110_complementary_ips.ps1.tmpl with parsed multi-IP entries.
func injectComplementaryIPTemplate(staticIPs string) ([]byte, error) {
	tmplContent, err := readScript("110_complementary_ips.ps1.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	macMap, warnings := staticip.ParseEntries(staticIPs)
	if len(warnings) > 0 {
		fmt.Printf("Warning: %d issue(s) parsing static IP entries\n", len(warnings))
	}
	configs := staticip.BuildComplementaryConfigs(macMap)

	rendered, err := staticip.RenderComplementaryTemplate(configs, tmplContent)
	if err != nil {
		return nil, fmt.Errorf("failed to render complementary template: %w", err)
	}
	return rendered, nil
}
