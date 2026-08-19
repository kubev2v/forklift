package vsphere

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	"github.com/yaacov/kc-utils/pkg/common/types"
	"github.com/yaacov/kc-utils/pkg/v2v/config"
)

// Inventory is VM metadata queried from vCenter (disks, NICs, firmware, guest hints).
type Inventory struct {
	Moref        string // Managed Object Reference value (e.g. vm-1052)
	Disks        []string
	NICs         []types.NICSpec
	FirmwareHint string
	GuestID      string
	GuestName    string
	HostName     string
}

var cache struct {
	sync.Mutex
	key string
	inv *Inventory
	err error
}

// LoadInventory queries vCenter using V2V_libvirtURL and V2V_vmName.
// Results are cached for the process (copy then conversion in the same pod).
func LoadInventory(cfg *config.Config) (*Inventory, error) {
	if cfg.LibvirtURL == "" || cfg.VmName == "" {
		return nil, fmt.Errorf("V2V_libvirtURL and V2V_vmName are required for vSphere inventory")
	}
	key := cfg.LibvirtURL + "\x00" + cfg.VmName + "\x00" + cfg.Fingerprint
	cache.Lock()
	defer cache.Unlock()
	if cache.inv != nil && cache.key == key {
		if cache.inv != nil {
			slog.Debug("vSphere inventory cache hit",
				"vm", cfg.VmName,
				"moref", cache.inv.Moref,
				"disks", len(cache.inv.Disks),
			)
		}
		return cache.inv, cache.err
	}
	cache.inv, cache.err = loadInventory(context.Background(), cfg)
	cache.key = key
	return cache.inv, cache.err
}

func loadInventory(ctx context.Context, cfg *config.Config) (*Inventory, error) {
	client, err := connect(ctx, cfg.LibvirtURL, cfg.Fingerprint)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Logout(ctx) }()

	finder := find.NewFinder(client.Client, true)
	if dcName := datacenterName(cfg.LibvirtURL); dcName != "" {
		if dc, dcErr := finder.Datacenter(ctx, dcName); dcErr == nil {
			finder.SetDatacenter(dc)
		}
	}

	vm, err := finder.VirtualMachine(ctx, cfg.VmName)
	if err != nil {
		return nil, fmt.Errorf("find VM %q: %w", cfg.VmName, err)
	}

	var vmMo mo.VirtualMachine
	pc := property.DefaultCollector(client.Client)
	if err := pc.RetrieveOne(ctx, vm.Reference(), []string{
		"config.hardware.device",
		"config.firmware",
		"config.guestId",
		"guest.guestFullName",
		"guest.hostName",
	}, &vmMo); err != nil {
		return nil, fmt.Errorf("read VM properties: %w", err)
	}

	if vmMo.Config == nil {
		return nil, fmt.Errorf("VM %q has no config", cfg.VmName)
	}

	inv := &Inventory{
		Moref:        vm.Reference().Value,
		Disks:        disksFromDevices(vmMo.Config.Hardware.Device),
		NICs:         nicsFromDevices(vmMo.Config.Hardware.Device),
		FirmwareHint: firmwareHint(vmMo.Config.Firmware),
		GuestID:      vmMo.Config.GuestId,
	}
	if vmMo.Guest != nil {
		inv.GuestName = vmMo.Guest.GuestFullName
		inv.HostName = vmMo.Guest.HostName
	}
	if len(inv.Disks) == 0 {
		return nil, fmt.Errorf("no vmdk disks found for VM %q", cfg.VmName)
	}
	slog.Info("loaded vSphere inventory",
		"vm", cfg.VmName,
		"moref", inv.Moref,
		"disks", len(inv.Disks),
		"nics", len(inv.NICs),
		"firmware", inv.FirmwareHint,
		"guestId", inv.GuestID,
		"guestName", inv.GuestName,
	)
	for i, d := range inv.Disks {
		slog.Info("vSphere source disk", "index", i, "path", d)
	}
	return inv, nil
}

func nicsFromDevices(devices []vimtypes.BaseVirtualDevice) []types.NICSpec {
	var nics []types.NICSpec
	for _, dev := range devices {
		mac := nicMAC(dev)
		if mac == "" {
			continue
		}
		nics = append(nics, types.NICSpec{MAC: mac})
	}
	return nics
}

func nicMAC(dev vimtypes.BaseVirtualDevice) string {
	switch nic := dev.(type) {
	case *vimtypes.VirtualVmxnet3:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualVmxnet2:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualVmxnet:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualE1000:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualE1000e:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualPCNet32:
		return strings.TrimSpace(nic.MacAddress)
	case *vimtypes.VirtualSriovEthernetCard:
		return strings.TrimSpace(nic.MacAddress)
	default:
		return ""
	}
}

func firmwareHint(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "efi", "uefi":
		return "uefi"
	default:
		return "bios"
	}
}

// ResetCache clears the inventory cache (tests).
func ResetCache() {
	cache.Lock()
	defer cache.Unlock()
	cache.key = ""
	cache.inv = nil
	cache.err = nil
}
