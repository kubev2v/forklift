package vmware

import (
	"context"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"k8s.io/klog/v2"
)

// HostClient is a govmomi session opened straight against an ESXi host's SDK
// endpoint rather than vCenter.
//
// The distinction matters: VirtualDiskManager.CopyVirtualDisk only offloads to
// VAAI XCOPY when the host itself services the call. Issued against vCenter the
// same request copies the data correctly but host-mediated, with no offload.
type HostClient struct {
	*govmomi.Client
	// Datacenter is the host's only datacenter, "ha-datacenter" on a standalone
	// ESXi. Resolved once at connect time since every disk operation needs it.
	Datacenter *object.Datacenter
	// Host is the single HostSystem served by this endpoint.
	Host *object.HostSystem
}

// NewHostClient logs in to the SDK endpoint of a single ESXi host.
//
// address is taken from the Host CR's spec.ipAddress; there is deliberately no
// fallback to the management IP, so the copy traffic stays on whichever network
// the Host CR designates.
func NewHostClient(ctx context.Context, address, username, password string) (*HostClient, error) {
	u, err := soap.ParseURL(address)
	if err != nil {
		return nil, fmt.Errorf("failed parsing ESXi host address %q: %w", address, err)
	}
	u.User = url.UserPassword(username, password)

	soapClient := soap.NewClient(u, true)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("failed creating ESXi host client for %q: %w", address, err)
	}
	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, sessionKeepAliveIdle)

	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}
	if err = c.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("failed to log in to ESXi host %q: %w", address, err)
	}

	finder := find.NewFinder(c.Client, true)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		_ = c.Logout(ctx)
		return nil, fmt.Errorf("failed to find datacenter on ESXi host %q: %w", address, err)
	}
	finder.SetDatacenter(dc)

	host, err := finder.DefaultHostSystem(ctx)
	if err != nil {
		_ = c.Logout(ctx)
		return nil, fmt.Errorf("failed to find host system on ESXi host %q: %w", address, err)
	}

	return &HostClient{Client: c, Datacenter: dc, Host: host}, nil
}

// ResolveDeviceName refreshes the host's storage view and returns the device
// name the host itself reports for the given canonical name (naa.X).
//
// Two reasons not to just build "/vmfs/devices/disks/<naa>" by hand: the host is
// the authority on the spelling, and hostd caches its storage inventory
// separately from the VMkernel. A LUN mapped and rescanned through vCenter esxcli
// is visible to esxcli while hostd still rejects it as an unknown device, so the
// refresh here is what makes a freshly mapped LUN usable as an RDM backing.
func (c *HostClient) ResolveDeviceName(ctx context.Context, canonicalName string) (string, error) {
	hss, err := c.Host.ConfigManager().StorageSystem(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get host storage system: %w", err)
	}

	// Best effort: a refresh failure is not fatal if the device is already known.
	if err := hss.Refresh(ctx); err != nil {
		klog.FromContext(ctx).V(2).Info("host storage refresh failed", "err", err)
	}

	var sys mo.HostStorageSystem
	if err := hss.Properties(ctx, hss.Reference(), []string{"storageDeviceInfo"}, &sys); err != nil {
		return "", fmt.Errorf("failed to read host storage device info: %w", err)
	}
	if sys.StorageDeviceInfo == nil {
		return "", fmt.Errorf("host reported no storage device info")
	}

	for _, lun := range sys.StorageDeviceInfo.ScsiLun {
		d := lun.GetScsiLun()
		if d.CanonicalName == canonicalName {
			return d.DeviceName, nil
		}
	}

	return "", fmt.Errorf(
		"device %s is not visible to the ESXi host's storage inventory (%d SCSI LUNs known); "+
			"the LUN may be mapped but not yet rescanned by hostd",
		canonicalName, len(sys.StorageDeviceInfo.ScsiLun))
}

// Close logs the session out. Errors are not actionable by the caller.
func (c *HostClient) Close(ctx context.Context) {
	if c == nil || c.Client == nil {
		return
	}
	_ = c.Logout(ctx)
}
