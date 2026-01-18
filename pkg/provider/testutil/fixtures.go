package testutil

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// PlanBuilder provides a fluent interface for building test Plan objects.
type PlanBuilder struct {
	plan *api.Plan
}

// NewPlanBuilder creates a new PlanBuilder with default values.
func NewPlanBuilder() *PlanBuilder {
	return &PlanBuilder{
		plan: &api.Plan{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-plan",
				Namespace: "test",
				UID:       k8stypes.UID("plan-uid-123"),
			},
			Spec: api.PlanSpec{
				TargetNamespace:                "test",
				PVCNameTemplateUseGenerateName: true,
			},
		},
	}
}

// WithName sets the plan name.
func (b *PlanBuilder) WithName(name string) *PlanBuilder {
	b.plan.Name = name
	return b
}

// WithNamespace sets the plan namespace.
func (b *PlanBuilder) WithNamespace(namespace string) *PlanBuilder {
	b.plan.Namespace = namespace
	b.plan.Spec.TargetNamespace = namespace
	return b
}

// WithTargetNamespace sets the target namespace for migrated VMs.
func (b *PlanBuilder) WithTargetNamespace(namespace string) *PlanBuilder {
	b.plan.Spec.TargetNamespace = namespace
	return b
}

// WithUID sets the plan UID.
func (b *PlanBuilder) WithUID(uid string) *PlanBuilder {
	b.plan.UID = k8stypes.UID(uid)
	return b
}

// WithVM adds a VM to the plan.
func (b *PlanBuilder) WithVM(name, id string) *PlanBuilder {
	b.plan.Spec.VMs = append(b.plan.Spec.VMs, planapi.VM{
		Ref: ref.Ref{Name: name, ID: id},
	})
	return b
}

// WithVMs adds multiple VMs to the plan.
func (b *PlanBuilder) WithVMs(vms ...planapi.VM) *PlanBuilder {
	b.plan.Spec.VMs = append(b.plan.Spec.VMs, vms...)
	return b
}

// WithSourceProvider sets the source provider reference.
func (b *PlanBuilder) WithSourceProvider(provider *api.Provider) *PlanBuilder {
	b.plan.Referenced.Provider.Source = provider
	return b
}

// WithDestinationProvider sets the destination provider reference.
func (b *PlanBuilder) WithDestinationProvider(provider *api.Provider) *PlanBuilder {
	b.plan.Referenced.Provider.Destination = provider
	return b
}

// WithNetworkMap sets the network map reference.
func (b *PlanBuilder) WithNetworkMap(networkMap *api.NetworkMap) *PlanBuilder {
	b.plan.Referenced.Map.Network = networkMap
	return b
}

// WithStorageMap sets the storage map reference.
func (b *PlanBuilder) WithStorageMap(storageMap *api.StorageMap) *PlanBuilder {
	b.plan.Referenced.Map.Storage = storageMap
	return b
}

// WithMigrationType sets the migration type (cold, warm).
func (b *PlanBuilder) WithMigrationType(migrationType api.MigrationType) *PlanBuilder {
	b.plan.Spec.Type = migrationType
	return b
}

// WithMigrationHistory adds a migration snapshot to the plan history.
func (b *PlanBuilder) WithMigrationHistory(migrationUID string) *PlanBuilder {
	b.plan.Status.Migration.History = append(b.plan.Status.Migration.History, planapi.Snapshot{
		Migration: planapi.SnapshotRef{
			UID: k8stypes.UID(migrationUID),
		},
	})
	return b
}

// Build returns the constructed Plan.
func (b *PlanBuilder) Build() *api.Plan {
	return b.plan
}

// ProviderBuilder provides a fluent interface for building test Provider objects.
type ProviderBuilder struct {
	provider *api.Provider
}

// NewProviderBuilder creates a new ProviderBuilder with default values.
func NewProviderBuilder() *ProviderBuilder {
	providerType := api.Undefined
	return &ProviderBuilder{
		provider: &api.Provider{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-provider",
				Namespace: "test",
				UID:       k8stypes.UID("provider-uid-123"),
			},
			Spec: api.ProviderSpec{
				Type:     &providerType,
				Settings: map[string]string{},
			},
		},
	}
}

// WithName sets the provider name.
func (b *ProviderBuilder) WithName(name string) *ProviderBuilder {
	b.provider.Name = name
	return b
}

// WithNamespace sets the provider namespace.
func (b *ProviderBuilder) WithNamespace(namespace string) *ProviderBuilder {
	b.provider.Namespace = namespace
	return b
}

// WithType sets the provider type.
func (b *ProviderBuilder) WithType(providerType api.ProviderType) *ProviderBuilder {
	b.provider.Spec.Type = &providerType
	return b
}

// WithURL sets the provider URL.
func (b *ProviderBuilder) WithURL(url string) *ProviderBuilder {
	b.provider.Spec.URL = url
	return b
}

// WithSecretRef sets the secret reference.
func (b *ProviderBuilder) WithSecretRef(name, namespace string) *ProviderBuilder {
	b.provider.Spec.Secret = core.ObjectReference{
		Name:      name,
		Namespace: namespace,
	}
	return b
}

// WithSetting adds a setting to the provider.
func (b *ProviderBuilder) WithSetting(key, value string) *ProviderBuilder {
	if b.provider.Spec.Settings == nil {
		b.provider.Spec.Settings = map[string]string{}
	}
	b.provider.Spec.Settings[key] = value
	return b
}

// WithSettings sets multiple settings, replacing any previously configured settings.
// Use WithSetting to add individual settings without overriding.
func (b *ProviderBuilder) WithSettings(settings map[string]string) *ProviderBuilder {
	b.provider.Spec.Settings = settings
	return b
}

// Build returns the constructed Provider.
func (b *ProviderBuilder) Build() *api.Provider {
	return b.provider
}

// SecretBuilder provides a fluent interface for building test Secret objects.
type SecretBuilder struct {
	secret *core.Secret
}

// NewSecretBuilder creates a new SecretBuilder with default values.
func NewSecretBuilder() *SecretBuilder {
	return &SecretBuilder{
		secret: &core.Secret{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test",
			},
			Data: map[string][]byte{},
		},
	}
}

// WithName sets the secret name.
func (b *SecretBuilder) WithName(name string) *SecretBuilder {
	b.secret.Name = name
	return b
}

// WithNamespace sets the secret namespace.
func (b *SecretBuilder) WithNamespace(namespace string) *SecretBuilder {
	b.secret.Namespace = namespace
	return b
}

// WithData adds a key-value pair to the secret data.
func (b *SecretBuilder) WithData(key, value string) *SecretBuilder {
	if b.secret.Data == nil {
		b.secret.Data = map[string][]byte{}
	}
	b.secret.Data[key] = []byte(value)
	return b
}

// WithDataMap sets the entire data map.
func (b *SecretBuilder) WithDataMap(data map[string][]byte) *SecretBuilder {
	b.secret.Data = data
	return b
}

// Build returns the constructed Secret.
func (b *SecretBuilder) Build() *core.Secret {
	return b.secret
}

// MigrationBuilder provides a fluent interface for building test Migration objects.
type MigrationBuilder struct {
	migration *api.Migration
}

// NewMigrationBuilder creates a new MigrationBuilder with default values.
func NewMigrationBuilder() *MigrationBuilder {
	return &MigrationBuilder{
		migration: &api.Migration{
			ObjectMeta: meta.ObjectMeta{
				Name:      "test-migration",
				Namespace: "test",
				UID:       k8stypes.UID("migration-uid-123"),
			},
		},
	}
}

// WithName sets the migration name.
func (b *MigrationBuilder) WithName(name string) *MigrationBuilder {
	b.migration.Name = name
	return b
}

// WithNamespace sets the migration namespace.
func (b *MigrationBuilder) WithNamespace(namespace string) *MigrationBuilder {
	b.migration.Namespace = namespace
	return b
}

// WithUID sets the migration UID.
func (b *MigrationBuilder) WithUID(uid string) *MigrationBuilder {
	b.migration.UID = k8stypes.UID(uid)
	return b
}

// WithPlanRef sets the reference to the associated Plan.
func (b *MigrationBuilder) WithPlanRef(name, namespace string) *MigrationBuilder {
	b.migration.Spec.Plan = core.ObjectReference{
		Name:      name,
		Namespace: namespace,
	}
	return b
}

// WithPlan sets the Plan reference from a Plan object.
func (b *MigrationBuilder) WithPlan(plan *api.Plan) *MigrationBuilder {
	b.migration.Spec.Plan = core.ObjectReference{
		Name:      plan.Name,
		Namespace: plan.Namespace,
	}
	return b
}

// WithCancel adds VMs to the cancel list.
func (b *MigrationBuilder) WithCancel(vms ...ref.Ref) *MigrationBuilder {
	b.migration.Spec.Cancel = append(b.migration.Spec.Cancel, vms...)
	return b
}

// WithCutover sets the cutover time for warm migrations.
func (b *MigrationBuilder) WithCutover(cutover meta.Time) *MigrationBuilder {
	b.migration.Spec.Cutover = &cutover
	return b
}

// Build returns the constructed Migration.
func (b *MigrationBuilder) Build() *api.Migration {
	return b.migration
}
