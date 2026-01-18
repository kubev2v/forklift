package testutil

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ContextBuilder provides a fluent interface for building test plancontext.Context objects.
type ContextBuilder struct {
	client     client.Client
	plan       *api.Plan
	migration  *api.Migration
	provider   *api.Provider
	secret     *core.Secret
	networkMap *api.NetworkMap
	storageMap *api.StorageMap
	destClient client.Client
	log        logging.LevelLogger
	objs       []runtime.Object
}

// NewContextBuilder creates a new ContextBuilder with default values.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		log:  logging.WithName("test"),
		objs: []runtime.Object{},
	}
}

// WithClient sets the Kubernetes client.
func (b *ContextBuilder) WithClient(client client.Client) *ContextBuilder {
	b.client = client
	return b
}

// WithObjects adds runtime objects to the fake client.
func (b *ContextBuilder) WithObjects(objs ...runtime.Object) *ContextBuilder {
	b.objs = append(b.objs, objs...)
	return b
}

// WithPlan sets the Plan.
func (b *ContextBuilder) WithPlan(plan *api.Plan) *ContextBuilder {
	b.plan = plan
	return b
}

// WithMigration sets the Migration.
func (b *ContextBuilder) WithMigration(migration *api.Migration) *ContextBuilder {
	b.migration = migration
	return b
}

// WithSourceProvider sets the source Provider.
func (b *ContextBuilder) WithSourceProvider(provider *api.Provider) *ContextBuilder {
	b.provider = provider
	return b
}

// WithSecret sets the provider Secret.
func (b *ContextBuilder) WithSecret(secret *core.Secret) *ContextBuilder {
	b.secret = secret
	return b
}

// WithNetworkMap sets the NetworkMap.
func (b *ContextBuilder) WithNetworkMap(networkMap *api.NetworkMap) *ContextBuilder {
	b.networkMap = networkMap
	return b
}

// WithStorageMap sets the StorageMap.
func (b *ContextBuilder) WithStorageMap(storageMap *api.StorageMap) *ContextBuilder {
	b.storageMap = storageMap
	return b
}

// WithDestinationClient sets a separate client for the destination cluster.
func (b *ContextBuilder) WithDestinationClient(client client.Client) *ContextBuilder {
	b.destClient = client
	return b
}

// WithLogger sets the logger.
func (b *ContextBuilder) WithLogger(log logging.LevelLogger) *ContextBuilder {
	b.log = log
	return b
}

// Build creates the plancontext.Context with all configured options.
func (b *ContextBuilder) Build() *plancontext.Context {
	// Create fake client if not provided
	if b.client == nil {
		b.client = NewFakeClient(b.objs...)
	}

	// Create default plan if not provided, or copy the provided plan
	// to avoid mutating the caller's Plan when setting Referenced fields.
	var plan *api.Plan
	if b.plan == nil {
		plan = NewPlanBuilder().Build()
	} else {
		plan = b.plan.DeepCopy()
	}

	// Create default migration if not provided
	if b.migration == nil {
		b.migration = NewMigrationBuilder().Build()
	}

	// Set up provider reference in plan
	if b.provider != nil {
		plan.Referenced.Provider.Source = b.provider
	}

	// Set up network map reference in plan
	if b.networkMap != nil {
		plan.Referenced.Map.Network = b.networkMap
	}

	// Set up storage map reference in plan
	if b.storageMap != nil {
		plan.Referenced.Map.Storage = b.storageMap
	}

	// Build the context
	ctx := &plancontext.Context{
		Client:    b.client,
		Plan:      plan,
		Migration: b.migration,
		Log:       b.log,
	}

	// Set up source
	ctx.Source.Provider = b.provider
	ctx.Source.Secret = b.secret

	// Set up destination
	if b.destClient != nil {
		ctx.Destination.Client = b.destClient
	} else {
		ctx.Destination.Client = b.client
	}

	// Set map references
	ctx.Map.Network = b.networkMap
	ctx.Map.Storage = b.storageMap

	return ctx
}

// NewTestContext creates a minimal test context with common defaults.
// Use this for simple tests that don't need extensive configuration.
func NewTestContext(objs ...runtime.Object) *plancontext.Context {
	return NewContextBuilder().
		WithObjects(objs...).
		Build()
}

// NewTestContextWithProvider creates a test context with a source provider and secret.
func NewTestContextWithProvider(provider *api.Provider, secret *core.Secret, objs ...runtime.Object) *plancontext.Context {
	return NewContextBuilder().
		WithSourceProvider(provider).
		WithSecret(secret).
		WithObjects(objs...).
		Build()
}
