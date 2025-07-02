package ova

import (
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// OVA VM Client
type Client struct {
	*plancontext.Context
	URL    string
	client *libweb.Client
	Secret *core.Secret
	Log    logr.Logger
}

// Connect to the OVA provider server.
func (r *Client) connect() (err error) {
	if r.client != nil {
		return
	}
	URL := r.Source.Provider.Spec.URL
	client := &libweb.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 15 * time.Second,
			}).DialContext,
			MaxIdleConns: 10,
		},
	}
	r.URL = URL
	r.client = client

	return
}

// Create a VM snapshot and return its ID. No-op for this provider.
func (r *Client) CreateSnapshot(vmRef ref.Ref, hostsFunc util.HostsFunc) (snapshotId string, creationTaskId string, err error) {
	return
}

// Remove a VM snapshot. No-op for this provider.
func (r *Client) RemoveSnapshot(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (removeTaskId string, err error) {
	return
}

// Get disk deltas for a VM snapshot. No-op for this provider.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (s map[string]string, err error) {
	return
}

// Check if a snapshot is ready to transfer, to avoid importer restarts.
func (r *Client) CheckSnapshotReady(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (ready bool, snapshotId string, err error) {
	return
}

// CheckSnapshotRemove implements base.Client
func (r *Client) CheckSnapshotRemove(vmRef ref.Ref, precopy planapi.Precopy, hosts util.HostsFunc) (bool, error) {
	return false, nil
}

// Set DataVolume checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) (err error) {
	return
}

// Get the power state of the VM.
func (r *Client) PowerState(vmRef ref.Ref) (state planapi.VMPowerState, err error) {
	return
}

// Power on the VM.
func (r *Client) PowerOn(vmRef ref.Ref) (err error) {
	return
}

// Power off the VM.
func (r *Client) PowerOff(vmRef ref.Ref) (err error) {
	return
}

// Determine whether the VM has been powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (poweredOff bool, err error) {
	return true, nil
}

// Close the connection to the OVA provider server.
func (r *Client) Close() {
	if r.client != nil {
		r.client = nil
	}
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	return
}

func (r Client) Finalize(vms []*planapi.VMStatus, planName string) {
}

func (r *Client) PreTransferActions(vmRef ref.Ref) (ready bool, err error) {
	ready = true
	return
}
