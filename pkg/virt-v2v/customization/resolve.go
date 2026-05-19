package customization

import (
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/boot/conversiondone"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/boot/diskrestore"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/boot/firstboot"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/drivers/vmware"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/network/netsh"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/network/registry"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/network/udev"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/plugins/scripts/dynamic"
)

// AllPlugins returns the ordered set of all known plugins.
// Ordering matters: network config before boot scripts, etc.
func AllPlugins() []api.Plugin {
	return []api.Plugin{
		// Linux: write udev rules for persistent NIC naming (replaces ifcfg/nm/netplan/ifquery/wicked)
		&udev.Plugin{},
		// Windows: configure static IPs via registry (modern drivers)
		&registry.Plugin{},
		// Windows: configure static IPs via netsh firstboot script (legacy drivers)
		&netsh.Plugin{},
		// Windows: remove VMware Tools drivers/services after vSphere conversion
		&vmware.Plugin{},
		// All: upload user-supplied custom scripts from ConfigMap
		&dynamic.Plugin{},
		// Windows: .bat orchestrator that runs all .ps1 firstboot scripts in order
		&firstboot.Plugin{},
		// Windows: restore offline VirtIO disks via WMI (legacy drivers only)
		&diskrestore.Plugin{},
		// Windows: write CONVERSION_DONE to COM1 so the controller knows boot finished
		&conversiondone.Plugin{},
	}
}

// Resolve filters AllPlugins() by calling Applicable(ctx) on each; order is preserved.
func Resolve(ctx *api.Context) []api.Plugin {
	if ctx == nil {
		return []api.Plugin{}
	}
	all := AllPlugins()
	var applicable []api.Plugin
	for _, p := range all {
		if p.Applicable(ctx) {
			applicable = append(applicable, p)
		}
	}
	return applicable
}
