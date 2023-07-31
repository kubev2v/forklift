package ocp

import (
	"context"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Openshift adapter.
type Adapter struct{}

// Constructs a openstack builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	builder = &Builder{Context: ctx}
	return
}

// Constructs a openshift validator.
func (r *Adapter) Validator(plan *api.Plan) (validator base.Validator, err error) {
	ref := plan.Provider.Source.Spec.Secret
	secret := &core.Secret{}

	conf, err := config.GetConfig()
	if err != nil {
		return
	}

	client, err := k8sclient.New(conf, k8sclient.Options{})
	if err != nil {
		return
	}

	// If the source is where we forklift runs we don't need the secret
	if plan.Provider.Source.IsHost() {
		secret = nil
	} else {
		err = client.Get(context.TODO(), k8sclient.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret)
		if err != nil {
			return
		}
	}

	sourceClient, err := plan.Provider.Source.Client(secret)
	if err != nil {
		return
	}

	v := &Validator{plan: plan, client: sourceClient}
	return v, nil
}

// Constructs an openshift client.
func (r *Adapter) Client(ctx *plancontext.Context) (client base.Client, err error) {
	client = &Client{Context: ctx}
	return client, nil
}

// Constucts a destination client.
func (r *Adapter) DestinationClient(ctx *plancontext.Context) (destinationClient base.DestinationClient, err error) {
	destinationClient = &DestinsationClient{Context: ctx}
	return
}
