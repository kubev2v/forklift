package dynamic

import (
	"errors"
	"net"
	"net/http"
	"time"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var (
	// ErrNotSupported is returned when a feature is not supported by dynamic providers
	ErrNotSupported = errors.New("operation not supported for dynamic providers")
)

// Client provides inventory access for dynamic providers.
type Client struct {
	*plancontext.Context
	URL    string
	client *libweb.Client
	Secret *core.Secret
}

// Connect establishes connection to the inventory service.
func (r *Client) Connect(secret *core.Secret) (err error) {
	r.URL = r.Source.Provider.Spec.URL
	r.Secret = secret

	// Build HTTP client
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	r.client = &libweb.Client{
		Transport: transport,
	}

	return
}

// PowerOn powers on a VM.
func (r *Client) PowerOn(vmRef ref.Ref) error {
	// Not supported for dynamic providers
	return nil
}

// PowerOff powers off a VM.
func (r *Client) PowerOff(vmRef ref.Ref) error {
	// Not supported for dynamic providers
	return nil
}

// PowerState returns VM power state.
func (r *Client) PowerState(vmRef ref.Ref) (planapi.VMPowerState, error) {
	return planapi.VMPowerStateUnknown, nil
}

// PoweredOff checks if VM is powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	return false, nil
}

// CreateSnapshot creates a VM snapshot.
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (snapshotId string, creationTaskId string, err error) {
	// Not supported for dynamic providers
	return "", "", nil
}

// RemoveSnapshot removes a VM snapshot.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (removeTaskId string, err error) {
	// Not supported for dynamic providers
	return "", nil
}

// CheckSnapshotReady checks if a snapshot is ready.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, snapshotId string, err error) {
	// Not supported for dynamic providers
	return false, "", nil
}

// CheckSnapshotRemove checks if a snapshot is removed.
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, err error) {
	// Not supported for dynamic providers
	return false, nil
}

// SetCheckpoints manages migration checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) (err error) {
	// Not supported for dynamic providers
	return
}

// Close closes the client connection.
func (r *Client) Close() {
	// No cleanup needed
}

// Finalize finalize migrations.
func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {
	// Not supported for dynamic providers
}

// DetachDisks detaches disks from target VM.
func (r *Client) DetachDisks(vmRef ref.Ref) error {
	// Not supported for dynamic providers
	return nil
}

// PreTransferActions performs pre-transfer actions.
func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	// Not supported for dynamic providers
	ready = true
	return
}

// GetSnapshotDeltas gets disk deltas for a snapshot.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (map[string]string, error) {
	// Not supported for dynamic providers
	return nil, ErrNotSupported
}
