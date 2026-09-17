package conversion

import "github.com/kubev2v/forklift/pkg/virt-v2v/utils"

// RunSourceV2vInspection inspects the vSphere source guest before conversion.
// virt-v2v-open opens the source via libvirt/VDDK and runs virt-inspector inside --run;
// @@ in the run command is expanded to -a <nbd-uri> per disk.
func (c *Conversion) RunSourceV2vInspection() error {
	return c.runVirtV2vOpenInspection(
		c.addVirtV2vOpenVsphereConnectionArgs,
		func(cmd utils.CommandBuilder) {
			cmd.AddPositional("--").AddPositional(c.VmName)
		},
	)
}
