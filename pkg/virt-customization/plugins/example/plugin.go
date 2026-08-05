// example/hello — Reference plugin demonstrating the Plugin interface.
//
// Applicable: Never (this is a documentation-only reference).
// Output:     FileAction{Write}, RegAction, ExecAction{Firstboot} — all illustrative.
//
// This plugin exists solely as a template for implementing real plugins.
// It shows how to:
//   - Implement the api.Plugin interface (Name, Applicable, Apply).
//   - Gate execution on guest OS type and config flags via Applicable.
//   - Return declarative Actions from Apply (files, registry, scripts).
//   - Embed static scripts using //go:embed.
//   - Follow the package-level doc header convention.
//
// To create a new plugin based on this example:
//  1. Copy this directory to plugins/<category>/<name>/.
//  2. Rename the package.
//  3. Implement Applicable with real conditions (OS family, config flags, etc.).
//  4. Implement Apply to return the actual actions your plugin needs.
//  5. Place any scripts in scripts/ and embed them with //go:embed scripts.
//  6. Register the plugin in resolve.go by adding it to AllPlugins() at the
//     correct position (ordering matters: network before drivers before boot).
//  7. Add tests in plugin_test.go covering Applicable and Apply.
package example

import (
	"embed"
	"fmt"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Embed the scripts/ directory so its contents are compiled into the binary.
// Each plugin that writes files to the guest should embed its scripts this way.
// Access individual files with scriptFS.ReadFile("scripts/<filename>").
//
//go:embed scripts
var scriptFS embed.FS

// readScript returns the content of the named file from the embedded scripts/ dir.
// Keeping this as a helper avoids repeating the "scripts/" prefix everywhere.
func readScript(name string) ([]byte, error) {
	return scriptFS.ReadFile("scripts/" + name)
}

// Plugin implements api.Plugin. Every plugin must be a struct (even if empty)
// so it can be registered as a pointer (&example.Plugin{}) in resolve.go.
type Plugin struct{}

// Name returns a unique, human-readable identifier for this plugin.
// Convention: "<category>/<name>" (e.g., "network/udev", "boot/windows/firstboot-runner").
func (p *Plugin) Name() string { return "example/hello" }

// Applicable decides whether this plugin should run for the given guest.
//
// Real plugins inspect ctx.Guest (OS family, detected network stacks, interfaces)
// and ctx.Config (feature flags, paths) to decide. For example:
//
//	// Only applies to Windows guests with static IPs configured.
//	func (p *Plugin) Applicable(ctx *api.Context) bool {
//	    if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
//	        return false
//	    }
//	    return ctx.Guest.OS.IsWindows() && ctx.Config.StaticIPs != ""
//	}
//
// This example always returns false since it is a reference, not a real plugin.
func (p *Plugin) Applicable(ctx *api.Context) bool {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return false
	}
	return false
}

// Apply collects the declarative actions this plugin wants performed on the guest.
//
// The returned *api.Actions contains three slices — real plugins populate only the
// ones they need:
//
//   - Files  []FileAction  — written to the guest disk via the GuestHandle (Phase 2).
//     Use ActionWrite for in-memory content, ActionUpload for host files.
//
//   - Regs   []RegAction   — merged into the Windows registry offline via
//     virt-win-reg --merge (Phase 3). Only relevant for Windows guests.
//
//   - Execs  []ExecAction  — executed via virt-customize (Phase 4).
//     Use ActionFirstboot for scripts that run on next boot,
//     ActionRun for scripts that run immediately in the offline guest.
//
// Plugins must NOT perform side effects here — only return data. The commit
// layer (commit/files.go, commit/registry.go, commit/scripts.go) materializes
// the actions onto the guest disk.
func (p *Plugin) Apply(ctx *api.Context) (*api.Actions, error) {
	if ctx == nil || ctx.Guest == nil || ctx.Config == nil {
		return nil, fmt.Errorf("example plugin: nil context, guest, or config")
	}

	// Read an embedded script. Real plugins embed .ps1, .bat, .sh, or .reg files.
	content, err := readScript("hello.sh")
	if err != nil {
		return nil, fmt.Errorf("read embedded script: %w", err)
	}

	return &api.Actions{
		// FileAction — write content directly into the guest filesystem.
		// The commit layer calls g.MkdirP on parent dirs, then g.Write or g.Upload.
		// GuestPath is the destination inside the guest disk. For example:
		//   - Windows firstboot: path.Join(api.WinFirstbootScriptsPath, "myscript.ps1")
		//   - Linux udev rules:  "/etc/udev/rules.d/70-persistent-net.rules"
		Files: []api.FileAction{
			{
				Type:        api.ActionWrite,
				GuestPath:   "/usr/local/bin/hello.sh",
				Content:     content,
				Permissions: "0755",
			},
		},

		// RegAction — offline Windows registry merge via virt-win-reg.
		// Content must be valid Windows REGEDIT (.reg) format.
		// Only include this for Windows-specific plugins.
		Regs: []api.RegAction{
			{
				Content: []byte("Windows Registry Editor Version 5.00\n\n" +
					"[HKEY_LOCAL_MACHINE\\SOFTWARE\\Example]\n" +
					"\"HelloKey\"=\"HelloValue\"\n"),
			},
		},

		// ExecAction — script execution via virt-customize CLI (Phase 4).
		// Use Content for inline scripts (the commit layer writes them to temp
		// files automatically). Use ScriptPath for pre-existing host files
		// (e.g., user-supplied scripts from a ConfigMap mount).
		// ActionFirstboot: registers the script to run on the guest's next boot.
		// ActionRun: runs the script immediately in the offline guest.
		Execs: []api.ExecAction{
			{
				Type:    api.ActionFirstboot,
				Content: content,
			},
		},
	}, nil
}
