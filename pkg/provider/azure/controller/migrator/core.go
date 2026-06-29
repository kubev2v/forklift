package migrator

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	azureadapter "github.com/kubev2v/forklift/pkg/provider/azure/controller/adapter"
	azureclient "github.com/kubev2v/forklift/pkg/provider/azure/controller/client"
	azureensurer "github.com/kubev2v/forklift/pkg/provider/azure/controller/ensurer"
)

type Migrator struct {
	*plancontext.Context
	log       logging.LevelLogger
	builder   base.Builder
	validator base.Validator
	vm        *planapi.VM
	adpClient base.Client
	ensurer   base.Ensurer
}

func New(ctx *plancontext.Context) (migbase.Migrator, error) {
	log := logging.WithName("migrator|azure")

	adp := azureadapter.New()

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

	azureCli, ok := client.(*azureclient.Client)
	if !ok {
		return nil, fmt.Errorf("failed to type assert client to *azureclient.Client, got %T", client)
	}

	if err = azureCli.Connect(); err != nil {
		log.Error(err, "Failed to connect Azure client")
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

func (r *Migrator) Type() api.MigrationType {
	return api.MigrationCold
}

// Supported returns true if the plan's migration type is compatible with this migrator.
func (r *Migrator) Supported() bool {
	return r.Context.Plan.Spec.Type == api.MigrationCold ||
		r.Context.Plan.Spec.Type == ""
}

func (r *Migrator) DestinationClient() base.Client {
	return nil
}

func (r *Migrator) SourceClient() base.Client {
	return nil
}

func (r *Migrator) SetSourceClient(client base.Client) {
}

func (r *Migrator) SetDestinationClient(client base.Client) {
}

func (r *Migrator) Logger() logging.LevelLogger {
	return r.log
}

func (r *Migrator) getEnsurer() *azureensurer.Ensurer {
	return r.ensurer.(*azureensurer.Ensurer)
}

func (r *Migrator) getAzureClient() *azureclient.Client {
	return r.adpClient.(*azureclient.Client)
}
