package client

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// PreTransferActions ensures the EC2 instance is stopped before data transfer begins.
// Checks if stopped, initiates shutdown if running, then verifies shutdown completed.
// Returns true when instance is confirmed stopped and ready for snapshotting.
func (r *Client) PreTransferActions(vmRef ref.Ref) (bool, error) {
	if _, err := r.getSourceClient(); err != nil {
		if connErr := r.Connect(); connErr != nil {
			return false, liberr.Wrap(connErr)
		}
	}

	isStopped, err := r.PoweredOff(vmRef)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if isStopped {
		return true, nil
	}

	if err := r.PowerOff(vmRef); err != nil {
		return false, liberr.Wrap(err)
	}

	isStopped, err = r.PoweredOff(vmRef)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	return isStopped, nil
}

// Finalize is a no-op for EC2 migrations.
// Cleanup is handled by the RemoveSnapshots phase instead of this method.
func (r *Client) Finalize(vms []*planapi.VMStatus, planName string) {
}
