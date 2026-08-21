package copy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/nfc"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	v2vtls "github.com/yaacov/kc-utils/pkg/v2v/tls"
	"github.com/yaacov/kc-utils/pkg/v2v/vsphere"
)

// DiskURL pairs a download URL with its source VMDK path and size.
type DiskURL struct {
	URL      string
	DiskPath string // [datastore] vm/disk.vmdk
	Size     int64  // bytes
}

// Lease wraps an NFC export lease and its per-disk download URLs.
type Lease struct {
	nfcLease *nfc.Lease
	client   *govmomi.Client
	DiskURLs []DiskURL
	cancel   context.CancelFunc
}

var diskTargetIDRE = regexp.MustCompile(`(?i)^disk-\d+\.vmdk$`)

// ExportVM starts an NFC export of the named VM and returns a Lease with
// per-disk HTTPS download URLs. The caller must call Complete or Abort.
func ExportVM(ctx context.Context, host, datacenter string, policy v2vtls.Policy, fingerprint, vmName string) (*Lease, error) {
	client, err := vsphere.ConnectHost(ctx, host, policy, fingerprint)
	if err != nil {
		return nil, err
	}

	finder := find.NewFinder(client.Client, true)
	if datacenter != "" {
		if dc, dcErr := finder.Datacenter(ctx, datacenter); dcErr == nil {
			finder.SetDatacenter(dc)
		}
	}

	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		_ = client.Logout(ctx)
		return nil, fmt.Errorf("find VM %q: %w", vmName, err)
	}

	lease, err := vm.Export(ctx)
	if err != nil {
		_ = client.Logout(ctx)
		return nil, fmt.Errorf("export VM %q: %w", vmName, err)
	}

	info, err := lease.Wait(ctx, nil)
	if err != nil {
		_ = client.Logout(ctx)
		return nil, fmt.Errorf("wait for NFC lease: %w", err)
	}

	devices, err := vm.Device(ctx)
	if err != nil {
		_ = lease.Abort(ctx, nil)
		_ = client.Logout(ctx)
		return nil, fmt.Errorf("read VM devices: %w", err)
	}

	diskURLs := mapDiskURLs(info, devices)
	slog.Info("NFC export lease acquired",
		"vm", vmName,
		"disks", len(diskURLs),
	)

	updaterCtx, cancel := context.WithCancel(ctx)
	go lease.StartUpdater(updaterCtx, info)

	return &Lease{
		nfcLease: lease,
		client:   client,
		DiskURLs: diskURLs,
		cancel:   cancel,
	}, nil
}

// Complete marks the lease as successfully finished.
func (l *Lease) Complete(ctx context.Context) error {
	l.cancel()
	err := l.nfcLease.Complete(ctx)
	_ = l.client.Logout(ctx)
	return err
}

// Abort cancels the lease.
func (l *Lease) Abort(ctx context.Context) error {
	l.cancel()
	err := l.nfcLease.Abort(ctx, nil)
	_ = l.client.Logout(ctx)
	return err
}

// mapDiskURLs pairs NFC disk device URLs with VMDK backing file paths.
// Non-disk lease items (nvram, etc.) are omitted. DiskPath is the VM backing
// FileName (normalized) so FilterDiskURLs can match inventory source_disks.
func mapDiskURLs(info *nfc.LeaseInfo, devices []vimtypes.BaseVirtualDevice) []DiskURL {
	if info == nil {
		return nil
	}

	devicePaths := map[string]string{}
	orderedPaths := orderedDiskBackingPaths(devices)
	for _, dev := range devices {
		disk, ok := dev.(*vimtypes.VirtualDisk)
		if !ok {
			continue
		}
		path := diskBackingPath(disk)
		if path == "" {
			continue
		}
		devicePaths[fmt.Sprintf("%d", disk.Key)] = path
	}

	n := len(info.DeviceUrl)
	if len(info.Items) < n {
		n = len(info.Items)
	}

	anyDiskFlag := false
	for i := 0; i < n; i++ {
		if info.DeviceUrl[i].Disk != nil {
			anyDiskFlag = true
			break
		}
	}

	type pending struct {
		url      string
		key      string
		targetID string
		size     int64
		diskPath string
	}
	var kept []pending
	for i := 0; i < n; i++ {
		device := info.DeviceUrl[i]
		item := info.Items[i]
		targetID := device.TargetId
		if targetID == "" {
			targetID = item.Path
		}
		if anyDiskFlag {
			if device.Disk == nil || !*device.Disk {
				continue
			}
		} else if !diskTargetIDRE.MatchString(targetID) {
			continue
		}

		itemURL := ""
		if item.URL != nil {
			itemURL = item.URL.String()
		}
		kept = append(kept, pending{
			url:      itemURL,
			key:      device.Key,
			targetID: targetID,
			size:     item.Size,
			diskPath: devicePaths[device.Key],
		})
	}

	used := map[string]bool{}
	for _, p := range kept {
		if p.diskPath != "" {
			used[p.diskPath] = true
		}
	}
	pos := 0
	for i := range kept {
		if kept[i].diskPath != "" {
			continue
		}
		for pos < len(orderedPaths) {
			path := orderedPaths[pos]
			pos++
			if used[path] {
				continue
			}
			kept[i].diskPath = path
			used[path] = true
			break
		}
		if kept[i].diskPath == "" {
			// Last resort: keep TargetId so logs show what NFC returned.
			kept[i].diskPath = kept[i].targetID
		}
	}

	urls := make([]DiskURL, 0, len(kept))
	for _, p := range kept {
		slog.Info("NFC lease disk",
			"key", p.key,
			"targetId", p.targetID,
			"size", p.size,
			"diskPath", p.diskPath,
		)
		urls = append(urls, DiskURL{
			URL:      p.url,
			DiskPath: p.diskPath,
			Size:     p.size,
		})
	}
	return urls
}

func diskBackingPath(disk *vimtypes.VirtualDisk) string {
	switch backing := disk.Backing.(type) {
	case *vimtypes.VirtualDiskFlatVer2BackingInfo:
		return normalizeDiskPath(flatBackingFileName(backing))
	case *vimtypes.VirtualDiskSparseVer2BackingInfo:
		return normalizeDiskPath(sparseBackingFileName(backing))
	case *vimtypes.VirtualDiskSeSparseBackingInfo:
		return normalizeDiskPath(backing.FileName)
	case *vimtypes.VirtualDiskRawDiskMappingVer1BackingInfo:
		return ""
	default:
		return ""
	}
}

func flatBackingFileName(backing *vimtypes.VirtualDiskFlatVer2BackingInfo) string {
	if backing == nil {
		return ""
	}
	if backing.Parent == nil {
		return backing.FileName
	}
	parent := backing.Parent
	for parent.Parent != nil {
		parent = parent.Parent
	}
	if parent.FileName != "" {
		return parent.FileName
	}
	return backing.FileName
}

func sparseBackingFileName(backing *vimtypes.VirtualDiskSparseVer2BackingInfo) string {
	if backing == nil {
		return ""
	}
	if backing.Parent == nil {
		return backing.FileName
	}
	parent := backing.Parent
	for parent.Parent != nil {
		parent = parent.Parent
	}
	if parent.FileName != "" {
		return parent.FileName
	}
	return backing.FileName
}

type copyDiskEntry struct {
	path          string
	bus           string
	controllerKey int32
	unitNumber    int32
	deviceKey     int32
}

var copyBusOrder = []string{"scsi", "sata", "ide", "nvme"}

func orderedDiskBackingPaths(devices []vimtypes.BaseVirtualDevice) []string {
	controllerBus := map[int32]string{}
	for _, dev := range devices {
		key := dev.GetVirtualDevice().Key
		if bus := copyControllerType(dev); bus != "" {
			controllerBus[key] = bus
		}
	}

	var entries []copyDiskEntry
	for _, dev := range devices {
		disk, ok := dev.(*vimtypes.VirtualDisk)
		if !ok {
			continue
		}
		path := diskBackingPath(disk)
		if path == "" {
			continue
		}
		vd := disk.GetVirtualDevice()
		bus := ""
		if vd.ControllerKey != 0 {
			bus = controllerBus[vd.ControllerKey]
		}
		unit := int32(-1)
		if vd.UnitNumber != nil {
			unit = *vd.UnitNumber
		}
		entries = append(entries, copyDiskEntry{
			path:          path,
			bus:           bus,
			controllerKey: vd.ControllerKey,
			unitNumber:    unit,
			deviceKey:     vd.Key,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if copyBusRank(a.bus) != copyBusRank(b.bus) {
			return copyBusRank(a.bus) < copyBusRank(b.bus)
		}
		if a.controllerKey != b.controllerKey {
			return a.controllerKey < b.controllerKey
		}
		if a.unitNumber != b.unitNumber {
			return a.unitNumber < b.unitNumber
		}
		return a.deviceKey < b.deviceKey
	})

	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.path)
	}
	return paths
}

func copyControllerType(dev vimtypes.BaseVirtualDevice) string {
	switch dev.(type) {
	case *vimtypes.VirtualLsiLogicController,
		*vimtypes.ParaVirtualSCSIController,
		*vimtypes.VirtualBusLogicController,
		*vimtypes.VirtualSCSIController:
		return "scsi"
	case *vimtypes.VirtualSATAController:
		return "sata"
	case *vimtypes.VirtualIDEController:
		return "ide"
	case *vimtypes.VirtualNVMEController:
		return "nvme"
	default:
		return ""
	}
}

func copyBusRank(bus string) int {
	for i, name := range copyBusOrder {
		if bus == name {
			return i
		}
	}
	return len(copyBusOrder)
}
