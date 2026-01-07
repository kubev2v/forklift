package migrator

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ec2adapter "github.com/kubev2v/forklift/pkg/provider/ec2/controller/adapter"
	ec2ensurer "github.com/kubev2v/forklift/pkg/provider/ec2/controller/ensurer"
)

// Migrator orchestrates EC2 to KubeVirt VM migrations through workflow phases.
// Flow: Initialize→PreHook→PowerOff→CreateSnapshots→WaitSnapshots→CreateDataVolumes→Finalize→CreateVM→RemoveSnapshots→PostHook→Complete
type Migrator struct {
	*plancontext.Context                     // Plan context with provider config, mappings, client
	log                  logging.LevelLogger // Structured logger
	builder              base.Builder        // Generates Kubernetes resource specs
	validator            base.Validator      // Validates migration prerequisites
	vm                   *planapi.VM         // Current VM being migrated
	adpClient            adapter.Client      // EC2 API operations (power, snapshots)
	ensurer              base.Ensurer        // Creates/verifies Kubernetes resources
}

// New creates and initializes EC2 Migrator with adapter components (builder, validator, ensurer, client).
// Connects EC2 client immediately, validating credentials and connectivity before migration starts.
func New(ctx *plancontext.Context) (migbase.Migrator, error) {
	log := logging.WithName("migrator|ec2")

	adp := ec2adapter.New()

	bldr, err := adp.Builder(ctx)
	if err != nil {
		log.Error(err, "Failed to get builder from adapter")
		return nil, err
	}

	validator, err := adp.Validator(ctx)
	if err != nil {
		log.Error(err, "Failed to get validator from adapter")
		return nil, err
	}

	client, err := adp.Client(ctx)
	if err != nil {
		log.Error(err, "Failed to get client from adapter")
		return nil, err
	}

	ens, err := adp.Ensurer(ctx)
	if err != nil {
		log.Error(err, "Failed to get ensurer from adapter")
		return nil, err
	}

	noopCli, ok := client.(*ec2adapter.NoopClient)
	if !ok {
		return nil, fmt.Errorf("failed to type assert client to *ec2adapter.NoopClient, got %T", client)
	}

	if err = noopCli.Client.Connect(); err != nil {
		log.Error(err, "Failed to connect EC2 client")
		return nil, err
	}

	migrator := &Migrator{
		Context:   ctx,
		log:       log,
		builder:   bldr,
		validator: validator,
		adpClient: client,
		ensurer:   ens,
	}

	return migrator, nil
}

// Type returns the supported migration type.
func (r *Migrator) Type() api.MigrationType {
	return api.MigrationCold
}

// Supported returns whether the plan's migration type is supported.
func (r *Migrator) Supported() bool {
	return r.Context.Plan.Spec.Type == api.MigrationCold ||
		r.Context.Plan.Spec.Type == ""
}

// DestinationClient returns the destination client (not used for EC2).
func (r *Migrator) DestinationClient() base.Client {
	return nil
}

// SourceClient returns the source client (not used for EC2).
func (r *Migrator) SourceClient() base.Client {
	return nil
}

// SetSourceClient sets the source client (not used for EC2).
func (r *Migrator) SetSourceClient(client base.Client) {
}

// SetDestinationClient sets the destination client (not used for EC2).
func (r *Migrator) SetDestinationClient(client base.Client) {
}

// Logger returns the logger.
func (r *Migrator) Logger() logging.LevelLogger {
	return r.log
}

// getEnsurer returns the EC2 ensurer.
func (r *Migrator) getEnsurer() *ec2ensurer.Ensurer {
	return r.ensurer.(*ec2ensurer.Ensurer)
}

// getEC2Client returns the EC2-specific client for direct AWS API operations.
func (r *Migrator) getEC2Client() *ec2adapter.NoopClient {
	return r.adpClient.(*ec2adapter.NoopClient)
}
