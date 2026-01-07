package adapter

import (
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/client"
	ec2ensurer "github.com/kubev2v/forklift/pkg/provider/ec2/controller/ensurer"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/validator"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// NoopClient embeds client.Client and provides no-op implementations for warm migration methods.
//
// EC2 migrations only support cold migration, so warm migration features like checkpoints
// and incremental snapshot deltas are not applicable. This wrapper provides no-op
// implementations of those methods while delegating supported operations to the embedded
// EC2 client.
type NoopClient struct {
	*client.Client
}

// SetCheckpoints is a no-op for EC2 - only cold migration supported, no checkpoints or incremental tracking.
func (r *NoopClient) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool, hostsFunc util.HostsFunc) error {
	return nil
}

// GetSnapshotDeltas is a no-op for EC2 - uses complete EBS snapshots, not incremental deltas.
func (r *NoopClient) GetSnapshotDeltas(vmRef ref.Ref, snapshot string, hostsFunc util.HostsFunc) (map[string]string, error) {
	return make(map[string]string), nil
}

// Compile-time interface checks. Ensures types implement required base interfaces.
// Catches missing/incorrect methods at compile time, prevents runtime panics.
var _ base.Adapter = &Adapter{}
var _ base.Client = &NoopClient{}
var _ base.DestinationClient = &DestinationClient{}
var _ base.Builder = &builder.Builder{}
var _ base.Ensurer = &ec2ensurer.Ensurer{}
var _ base.Validator = &validator.Validator{}
