package vsphere

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/lib/logging"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	annotationPrefix             = "forklift.konveyor.io/toehold-"
	DiskHashAnnotation           = annotationPrefix + "disk-hash"
	ConfigHashAnnotation         = annotationPrefix + "config-hash"
	BaseContainerImageAnnotation = annotationPrefix + "base-container-image"
	ImportedAtAnnotation         = annotationPrefix + "imported-at"
	// Rough floor for a new template upload; the real size comes from the base image.
	DefaultTemplateDatastoreFreeBytes = 10 * 1024 * 1024 * 1024
)

var log = logging.WithName("toehold|vsphere")

var templateAnnotationKeys = []string{
	DiskHashAnnotation,
	ConfigHashAnnotation,
	BaseContainerImageAnnotation,
	ImportedAtAnnotation,
}

// Client wraps a shared vSphere Session with toehold-specific helpers.
type Client struct {
	*libvsphere.Session
}

// VMRef holds a located virtual machine or template.
type VMRef = libvsphere.VMRef

// InventoryPreflight checks vCenter inventory before template build.
type InventoryPreflight struct {
	Folder    string
	Datastore string
	Network   string
	// MinFreeBytes, when > 0, requires the datastore to have at least that much free space.
	MinFreeBytes int64
}

// NewClient wraps an existing govmomi session.
func NewClient(gc *govmomi.Client) (*Client, error) {
	s, err := libvsphere.NewSession(context.Background(), gc, true)
	if err != nil {
		return nil, err
	}
	return &Client{Session: s}, nil
}

// ValidateInventory verifies folder, network, and datastore exist, are accessible,
// have enough free space when required, and have a suitable ESXi import host.
func (c *Client) ValidateInventory(ctx context.Context, pf InventoryPreflight) error {
	if pf.Datastore == "" {
		return fmt.Errorf("datastore name is required")
	}
	if _, err := c.findFolder(ctx, pf.Folder); err != nil {
		return fmt.Errorf("folder %q: %w", pf.Folder, err)
	}
	var net object.NetworkReference
	var err error
	if pf.Network != "" {
		net, err = c.Finder.Network(ctx, pf.Network)
		if err != nil {
			return fmt.Errorf("network %q: %w", pf.Network, err)
		}
	}
	datastore, err := c.Finder.Datastore(ctx, pf.Datastore)
	if err != nil {
		return fmt.Errorf("datastore %q: %w", pf.Datastore, err)
	}
	var props mo.Datastore
	if err = datastore.Properties(ctx, datastore.Reference(), []string{"summary"}, &props); err != nil {
		return fmt.Errorf("datastore %q: %w", pf.Datastore, err)
	}
	free, accessible := props.Summary.FreeSpace, props.Summary.Accessible
	if !accessible {
		return fmt.Errorf("datastore %q is not accessible", pf.Datastore)
	}
	if pf.MinFreeBytes > 0 && free < pf.MinFreeBytes {
		return fmt.Errorf(
			"datastore %q has %s free but at least %s is required",
			pf.Datastore,
			formatBytes(free),
			formatBytes(pf.MinFreeBytes),
		)
	}
	if _, err = c.findImportHost(ctx, datastore, net); err != nil {
		return fmt.Errorf("no suitable ESXi host for import on datastore %q: %w", pf.Datastore, err)
	}
	return nil
}

// GetAnnotationMap reads forklift toehold annotations from a VM/template.
func (c *Client) GetAnnotationMap(ctx context.Context, vm *object.VirtualMachine) (map[string]string, error) {
	var o mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config"}, &o); err != nil {
		return nil, err
	}
	result := map[string]string{}
	if o.Config == nil {
		return result, nil
	}
	parseAnnotations(o.Config.Annotation, result)
	for _, extra := range o.Config.ExtraConfig {
		if val, ok := extra.(*types.OptionValue); ok {
			if strings.HasPrefix(val.Key, annotationPrefix) {
				result[val.Key] = fmt.Sprint(val.Value)
			}
		}
	}
	return result, nil
}

// SetAnnotationMap writes forklift toehold annotations on a VM/template.
func (c *Client) SetAnnotationMap(ctx context.Context, vm *object.VirtualMachine, values map[string]string) error {
	var o mo.VirtualMachine
	if err := vm.Properties(ctx, vm.Reference(), []string{"config.template", "name"}, &o); err != nil {
		return err
	}
	isTemplate := o.Config != nil && o.Config.Template
	name := o.Name
	if name == "" {
		name = vm.Reference().Value
	}

	existing, err := c.GetAnnotationMap(ctx, vm)
	if err != nil {
		return err
	}
	for k, v := range values {
		existing[k] = v
	}
	spec := types.VirtualMachineConfigSpec{
		Annotation: encodeAnnotations(existing),
	}
	extraCount := 0
	// vCenter rejects ExtraConfig reconfigure on templates ("operation is not supported").
	if !isTemplate {
		extra := []types.BaseOptionValue{}
		for _, key := range templateAnnotationKeys {
			if v, ok := values[key]; ok {
				extra = append(extra, &types.OptionValue{Key: key, Value: v})
			}
		}
		spec.ExtraConfig = extra
		extraCount = len(extra)
	}
	log.V(1).Info("Setting annotations",
		"name", name,
		"moref", vm.Reference().Value,
		"isTemplate", isTemplate,
		"keys", len(values),
		"annotationBytes", len(spec.Annotation),
		"extraConfig", extraCount,
	)
	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("reconfigure %s (isTemplate=%v): %w", name, isTemplate, err)
	}
	if err = task.Wait(ctx); err != nil {
		return fmt.Errorf("reconfigure task %s (isTemplate=%v): %w", name, isTemplate, err)
	}
	log.V(1).Info("Set annotations complete", "name", name, "moref", vm.Reference().Value)
	return nil
}

// SetDiskEnableUUID enables disk.EnableUUID so guests can read stable SCSI
// identifiers that match VMware backing.Uuid values.
func (c *Client) SetDiskEnableUUID(ctx context.Context, vm *object.VirtualMachine) error {
	log.V(1).Info("Enabling disk.EnableUUID", "moref", vm.Reference().Value)
	spec := types.VirtualMachineConfigSpec{
		ExtraConfig: []types.BaseOptionValue{
			&types.OptionValue{Key: "disk.EnableUUID", Value: "TRUE"},
		},
	}
	task, err := vm.Reconfigure(ctx, spec)
	if err != nil {
		return fmt.Errorf("enable disk.EnableUUID reconfigure %s: %w", vm.Reference().Value, err)
	}
	if err = task.Wait(ctx); err != nil {
		return fmt.Errorf("enable disk.EnableUUID task %s: %w", vm.Reference().Value, err)
	}
	log.V(1).Info("Enabled disk.EnableUUID", "moref", vm.Reference().Value)
	return nil
}

func (c *Client) findFolder(ctx context.Context, folderPath string) (*object.Folder, error) {
	// Keep a leading "/": with Finder.SetDatacenter, "Datacenter/vm/child" fails
	// while "/Datacenter/vm/child" and "vm/child" succeed.
	path := strings.TrimSpace(folderPath)
	if path == "" {
		return c.Finder.DefaultFolder(ctx)
	}
	return c.Finder.Folder(ctx, path)
}

func (c *Client) findResourcePool(ctx context.Context, folderPath string) (*object.ResourcePool, error) {
	pool, err := c.Finder.ResourcePool(ctx, "Resources")
	if err == nil {
		return pool, nil
	}
	folder, err := c.findFolder(ctx, folderPath)
	if err != nil {
		return nil, err
	}
	children, err := folder.Children(ctx)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		ref := child.Reference()
		if ref.Type == "ResourcePool" {
			return object.NewResourcePool(c.Client.Client, ref), nil
		}
		if ref.Type == "ClusterComputeResource" || ref.Type == "ComputeResource" {
			cr := object.NewComputeResource(c.Client.Client, ref)
			p, perr := cr.ResourcePool(ctx)
			if perr == nil {
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("resource pool not found under %q", folderPath)
}

func (c *Client) findImportHost(ctx context.Context, datastore *object.Datastore, net object.NetworkReference) (*object.HostSystem, error) {
	hosts, err := datastore.AttachedHosts(ctx)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts attached to datastore %q", datastore.Name())
	}
	var netRef types.ManagedObjectReference
	if net != nil {
		netRef = net.Reference()
	}
	for _, host := range hosts {
		var o mo.HostSystem
		if err := host.Properties(ctx, host.Reference(), []string{"network", "runtime"}, &o); err != nil {
			continue
		}
		if o.Runtime.InMaintenanceMode ||
			o.Runtime.PowerState == types.HostSystemPowerStatePoweredOff ||
			o.Runtime.PowerState == types.HostSystemPowerStateStandBy {
			continue
		}
		if _, ok := net.(*object.Network); ok {
			found := false
			for _, n := range o.Network {
				if n.Value == netRef.Value {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		return host, nil
	}
	log.V(1).Info("No host matched network/runtime filters; using fallback host",
		"host", hostDisplayName(ctx, hosts[0]),
		"moref", hosts[0].Reference().Value,
	)
	return hosts[0], nil
}

func formatBytes(n int64) string {
	const gib = 1024 * 1024 * 1024
	if n >= gib {
		return fmt.Sprintf("%.1fGiB", float64(n)/float64(gib))
	}
	const mib = 1024 * 1024
	if n >= mib {
		return fmt.Sprintf("%.1fMiB", float64(n)/float64(mib))
	}
	return fmt.Sprintf("%dB", n)
}

func hostDisplayName(ctx context.Context, host *object.HostSystem) string {
	var o mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"name"}, &o); err != nil {
		return host.Reference().Value
	}
	if o.Name == "" {
		return host.Reference().Value
	}
	return o.Name
}

func parseAnnotations(annotation string, out map[string]string) {
	for _, line := range strings.Split(annotation, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
}

func encodeAnnotations(values map[string]string) string {
	lines := []string{}
	for _, key := range templateAnnotationKeys {
		if v, ok := values[key]; ok && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", key, v))
		}
	}
	return strings.Join(lines, "\n")
}
