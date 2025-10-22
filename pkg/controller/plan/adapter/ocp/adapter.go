package ocp

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Openshift adapter.
type Adapter struct{}

// Constructs a openstack builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	sourceClient, err := createClient(ctx.Source.Provider)
	if err != nil {
		return
	}

	builder = &Builder{Context: ctx, sourceClient: sourceClient}
	return
}

// Constructs a openshift validator.
func (r *Adapter) Validator(plan *api.Plan) (validator base.Validator, err error) {
	sourceClient, err := createClient(plan.Provider.Source)
	if err != nil {
		return
	}

	log := logging.WithName("validator|ocp").WithValues(
		"plan",
		path.Join(
			plan.GetNamespace(),
			plan.GetName()))

	validator = &Validator{plan: plan, sourceClient: sourceClient, log: log}
	return
}

// Constructs an openshift client.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	sourceClient, err := createClient(ctx.Source.Provider)
	if err != nil {
		return
	}

	client = &Client{Context: ctx, sourceClient: sourceClient}

	return
}

// Constucts a destination client.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (destinationClient base.DestinationClient, err error) {
	destinationClient = &DestinationClient{Context: ctx}
	return
}

func createClient(sourceProvider *api.Provider) (sourceClient k8sclient.Client, err error) {
	conf, err := config.GetConfig()
	if err != nil {
		return
	}

	client, err := k8sclient.New(conf, k8sclient.Options{})
	if err != nil {
		return
	}

	if sourceProvider.IsHost() {
		sourceClient = client
	} else {
		ref := sourceProvider.Spec.Secret
		secret := &core.Secret{}
		err = client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret)
		if err != nil {
			return
		}

		sourceClient, err = ocp.Client(sourceProvider, secret)
		if err != nil {
			return
		}
	}

	return
}
