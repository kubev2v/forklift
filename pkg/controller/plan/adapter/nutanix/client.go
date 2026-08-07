package nutanix

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Client performs source-side Nutanix migration actions.
type Client struct {
	*plancontext.Context
	log logging.LevelLogger
}

func (r *Client) connect() error {
	r.log = r.Log.WithName("client")
	// TODO: wire container/nutanix.Client for PreTransferActions and power management
	return nil
}

func (r *Client) Close() {}

func (r *Client) Finalize(_ []*planapi.VMStatus, _ string) {
	// TODO: delete temporary catalog images created during PreTransferActions
}

func (r *Client) DetachDisks(_ ref.Ref) error {
	return nil
}

func (r *Client) PowerState(_ ref.Ref) (planapi.VMPowerState, error) {
	// TODO: read from inventory (ON/OFF) or Nutanix API once web client is wired
	return planapi.VMPowerStateUnknown, nil
}

func (r *Client) PowerOn(_ ref.Ref) error {
	// TODO: Nutanix VM power-on API
	return nil
}

func (r *Client) PowerOff(_ ref.Ref) error {
	// TODO: Nutanix VM power-off API
	return nil
}

func (r *Client) PoweredOff(_ ref.Ref) (bool, error) {
	// TODO: poll Nutanix API or refreshed inventory after PowerOff
	return false, nil
}

func (r *Client) CreateSnapshot(_ ref.Ref, _ util.HostsFunc) (string, string, error) {
	return "", "", nil
}

func (r *Client) RemoveSnapshot(_ ref.Ref, _ string, _ util.HostsFunc) (string, error) {
	return "", nil
}

func (r *Client) CheckSnapshotReady(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, string, error) {
	return true, "", nil
}

func (r *Client) CheckSnapshotRemove(_ ref.Ref, _ planapi.Precopy, _ util.HostsFunc) (bool, error) {
	return true, nil
}

func (r *Client) SetCheckpoints(_ ref.Ref, _ []planapi.Precopy, _ []cdi.DataVolume, _ bool, _ util.HostsFunc) error {
	return nil
}

func (r *Client) GetSnapshotDeltas(_ ref.Ref, _ string, _ util.HostsFunc) (s map[string]string, err error) {
	return
}

func (r *Client) PreTransferActions(_ ref.Ref) (ready bool, err error) {
	// TODO: create catalog images from VM disks and wait until COMPLETE
	return true, nil
}
