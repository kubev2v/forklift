package customization_test

import (
	"os"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeFS struct{}

func (f *fakeFS) Symlink(_, _ string) error                         { return nil }
func (f *fakeFS) Stat(_ string) (os.FileInfo, error)                { return nil, os.ErrNotExist }
func (f *fakeFS) WriteFile(_ string, _ []byte, _ os.FileMode) error { return nil }
func (f *fakeFS) ReadDir(_ string) ([]os.DirEntry, error)           { return nil, os.ErrNotExist }

func TestPostprocess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Customize Suite")
}

func linuxGuest(distro string, interfaces []api.InterfaceInfo) *api.GuestInfo {
	return &api.GuestInfo{
		OS:         api.GuestOS{Family: api.OSFamilyLinux, Distro: distro},
		Interfaces: interfaces,
	}
}

func windowsGuest() *api.GuestInfo {
	return &api.GuestInfo{
		OS: api.GuestOS{Family: api.OSFamilyWindows},
	}
}

var sampleInterfaces = []api.InterfaceInfo{
	{Name: "eth0", IPv4: []string{"192.168.1.100"}, Source: "ifcfg"},
}

var _ = Describe("Resolve", func() {
	It("selects network/udev for Linux with static IPs and interfaces", func() {
		ctx := &api.Context{
			Guest:      linuxGuest("rhel", sampleInterfaces),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs: "00:11:22:33:44:55:ip:192.168.1.100",
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("network/udev"))
		Expect(names).NotTo(ContainElement("boot/windows/firstboot-runner"))
	})

	It("does not select network/udev when no interfaces extracted", func() {
		ctx := &api.Context{
			Guest:      linuxGuest("rhel", nil),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs: "00:11:22:33:44:55:ip:192.168.1.100",
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).NotTo(ContainElement("network/udev"))
	})

	It("selects Windows registry plugin when flag is set", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs:                    "00:11:22:33:44:55:ip:192.168.1.100",
				WindowsRegistryNetworkConfig: true,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("network/registry"))
		Expect(names).NotTo(ContainElement("network/netsh"))
		Expect(names).To(ContainElement("boot/windows/firstboot-runner"))
	})

	It("selects Windows netsh plugin for legacy drivers", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs:              "00:11:22:33:44:55:ip:192.168.1.100",
				VirtIoWinLegacyDrivers: "/mnt/virtio.iso",
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("network/netsh"))
		Expect(names).NotTo(ContainElement("network/registry"))
	})

	It("selects VMware driver removal for Windows + vSphere", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				VsphereVmwareDriverRemoval: true,
				Source:                     config.VSPHERE,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("drivers/vmware"))
	})

	It("does not select VMware driver removal for non-vSphere", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				VsphereVmwareDriverRemoval: true,
				Source:                     config.OVA,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).NotTo(ContainElement("drivers/vmware"))
	})

	It("does not include LUKS as a plugin (keys are infrastructure)", func() {
		ctx := &api.Context{
			Guest:      linuxGuest("rhel", nil),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				Luksdir: "/etc/luks",
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).NotTo(ContainElement("encryption/luks"))
	})

	It("skips network plugins when no static IPs", func() {
		ctx := &api.Context{
			Guest:      linuxGuest("rhel", sampleInterfaces),
			FileSystem: &fakeFS{},
			Config:     &config.AppConfig{},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).NotTo(ContainElement("network/udev"))
	})

	It("selects conversion-done when WaitForGuestReboot is true", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				WaitForGuestReboot: true,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("boot/windows/conversion-done"))
	})

	It("selects disk-restore for legacy drivers", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				VirtIoWinLegacyDrivers: "/mnt/virtio.iso",
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("boot/windows/disk-restore"))
	})

	It("registry wins over netsh when both flags are set", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs:                    "00:11:22:33:44:55:ip:192.168.1.100",
				VirtIoWinLegacyDrivers:       "/mnt/virtio.iso",
				WindowsRegistryNetworkConfig: true,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("network/registry"))
		Expect(names).NotTo(ContainElement("network/netsh"))
	})

	It("always selects firstboot-runner for Windows", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config:     &config.AppConfig{},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		Expect(names).To(ContainElement("boot/windows/firstboot-runner"))
	})

	It("returns no plugins for bare Linux with no config", func() {
		ctx := &api.Context{
			Guest:      linuxGuest("rhel", nil),
			FileSystem: &fakeFS{},
			Config:     &config.AppConfig{},
		}
		plugins := customization.Resolve(ctx)
		Expect(plugins).To(BeEmpty())
	})

	It("preserves plugin ordering: network before drivers before dynamic before boot", func() {
		ctx := &api.Context{
			Guest:      windowsGuest(),
			FileSystem: &fakeFS{},
			Config: &config.AppConfig{
				StaticIPs:                    "00:11:22:33:44:55:ip:192.168.1.100",
				WindowsRegistryNetworkConfig: true,
				VsphereVmwareDriverRemoval:   true,
				Source:                       config.VSPHERE,
				WaitForGuestReboot:           true,
			},
		}
		plugins := customization.Resolve(ctx)
		names := pluginNames(plugins)
		registryIdx := indexOf(names, "network/registry")
		vmwareIdx := indexOf(names, "drivers/vmware")
		firstbootIdx := indexOf(names, "boot/windows/firstboot-runner")
		conversionIdx := indexOf(names, "boot/windows/conversion-done")
		Expect(registryIdx).ToNot(Equal(-1))
		Expect(vmwareIdx).ToNot(Equal(-1))
		Expect(firstbootIdx).ToNot(Equal(-1))
		Expect(conversionIdx).ToNot(Equal(-1))
		Expect(registryIdx).To(BeNumerically("<", vmwareIdx))
		Expect(vmwareIdx).To(BeNumerically("<", firstbootIdx))
		Expect(firstbootIdx).To(BeNumerically("<", conversionIdx))
	})
})

func pluginNames(plugins []api.Plugin) []string {
	var names []string
	for _, p := range plugins {
		names = append(names, p.Name())
	}
	return names
}

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
