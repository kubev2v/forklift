package client

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Disconnect is a no-op for EC2 - AWS SDK clients are stateless, no persistent connections.
func (r *Client) Disconnect() error {
	return nil
}

// DetachDisks is a no-op for EC2 - snapshots created from attached volumes after instance shutdown.
func (r *Client) DetachDisks(vmRef ref.Ref) error {
	return nil
}

// SetCheckpoints is a no-op for EC2 - only cold migration supported, no checkpoints or incremental tracking.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) error {
	return nil
}

// GetSnapshotDeltas is a no-op for EC2 - uses complete EBS snapshots, not incremental deltas.
func (r *Client) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (map[string]string, error) {
	return make(map[string]string), nil
}

// Compile-time interface check. Ensures Client implements required base.Client interface.
var _ base.Client = &Client{}
