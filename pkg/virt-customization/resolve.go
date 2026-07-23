package customization

import (
	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	"github.com/kubev2v/forklift/pkg/virt-customization/plugins/example"
)

// AllPlugins returns the ordered set of all known plugins.
//
// Ordering matters: network config before drivers before boot scripts, etc.
// When adding real plugins, insert them at the correct position. For example:
//
//	return []api.Plugin{
//	    &udev.Plugin{},          // 1. Linux network
//	    &registry.Plugin{},      // 2. Windows network (modern)
//	    &netsh.Plugin{},         // 3. Windows network (legacy)
//	    &vmware.Plugin{},        // 4. Driver removal
//	    &dynamic.Plugin{},       // 5. User-supplied scripts
//	    &firstboot.Plugin{},     // 6. Boot orchestrator
//	    &diskrestore.Plugin{},   // 7. Disk restore
//	    &conversiondone.Plugin{},// 8. Conversion signal
//	}
func AllPlugins() []api.Plugin {
	return []api.Plugin{
		&example.Plugin{},
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
