package adapter

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/client"
	ec2ensurer "github.com/kubev2v/forklift/pkg/provider/ec2/controller/ensurer"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/validator"
)

// Adapter provides EC2-specific migration components that implement the Forklift migration
// framework interfaces. It serves as a factory for creating provider-specific implementations
// of builders, validators, ensurers, and clients needed for the migration process.
// This adapter is the main entry point for EC2 migration operations.
type Adapter struct{}

// New creates a new EC2 Adapter instance.
// This is the factory method for creating the EC2 provider adapter.
//
// Returns:
//   - *Adapter: A new adapter instance ready to provide EC2-specific migration components
func New() *Adapter {
	return &Adapter{}
}

// Ensurer returns EC2-specific ensurer for creating/managing Kubernetes resources during migration.
// Creates PVs, PVCs, and secrets with AWS credentials in target namespace.
func (r *Adapter) Ensurer(ctx *plancontext.Context) (base.Ensurer, error) {
	return ec2ensurer.New(ctx), nil
}

// Builder returns EC2-specific builder for generating Kubernetes resource specs from EC2 instances/volumes.
// Converts instances to VirtualMachine specs, volumes to PVs and PVCs, maps instance types.
func (r *Adapter) Builder(ctx *plancontext.Context) (base.Builder, error) {
	return builder.New(ctx), nil
}

// Validator returns EC2-specific validator for checking migration preconditions.
// Validates EBS volumes exist, detects unsupported instance store, checks network/storage mappings complete.
func (r *Adapter) Validator(ctx *plancontext.Context) (base.Validator, error) {
	return validator.New(ctx), nil
}

// Client returns EC2-specific client for instance operations: power management, snapshots, volumes.
// EC2 only supports cold migration, so warm migration methods (SetCheckpoints, GetSnapshotDeltas) are no-ops.
func (r *Adapter) Client(ctx *plancontext.Context) (base.Client, error) {
	return &client.Client{Context: ctx}, nil
}

// DestinationClient returns destination client for managing direct volume ownership.
// Ensures resources have proper owner references for automatic garbage collection.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (base.DestinationClient, error) {
	return &DestinationClient{Context: ctx}, nil
}
