package hyperv

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovfbase"
)

// Type aliases for backward compatibility.
// These allow existing code to continue using hyperv.VM, hyperv.Network, etc.

type (
	// Handler types
	Handler         = ovfbase.Handler
	ProviderHandler = ovfbase.ProviderHandler
	VMHandler       = ovfbase.VMHandler
	NetworkHandler  = ovfbase.NetworkHandler
	DiskHandler     = ovfbase.DiskHandler
	StorageHandler  = ovfbase.StorageHandler
	WorkloadHandler = ovfbase.WorkloadHandler
	TreeHandler     = ovfbase.TreeHandler

	// Resource types
	Resource = ovfbase.Resource
	Provider = ovfbase.Provider
	VM       = ovfbase.VM
	VM0      = ovfbase.VM0
	VM1      = ovfbase.VM1
	Network  = ovfbase.Network
	Disk     = ovfbase.Disk
	Storage  = ovfbase.Storage
	Workload = ovfbase.Workload

	// Utility types
	PathBuilder = ovfbase.PathBuilder
	Tree        = ovfbase.Tree
	TreeNode    = ovfbase.TreeNode

	// Error types
	ResourceNotResolvedError = ovfbase.ResourceNotResolvedError
	RefNotUniqueError        = ovfbase.RefNotUniqueError
	NotFoundError            = ovfbase.NotFoundError
)

// Resolver for HyperV providers - pre-configured with HyperV config.
type Resolver struct {
	*api.Provider
}

// Path builds the URL path for a resource.
func (r *Resolver) Path(resource interface{}, id string) (path string, err error) {
	resolver := &ovfbase.Resolver{
		Provider: r.Provider,
		Config:   Config,
	}
	return resolver.Path(resource, id)
}

// Finder for HyperV providers - pre-configured with HyperV config.
type Finder struct {
	ovfbase.Finder
}

// NewFinder creates a new HyperV finder with the correct config.
func NewFinder() *Finder {
	return &Finder{
		Finder: ovfbase.Finder{
			Config: Config,
		},
	}
}

// Constants re-exported for backward compatibility
const (
	ProviderParam      = ovfbase.ProviderParam
	VMParam            = ovfbase.VMParam
	VMCollection       = ovfbase.VMCollection
	NetworkParam       = ovfbase.NetworkParam
	NetworkCollection  = ovfbase.NetworkCollection
	DiskParam          = ovfbase.DiskParam
	DiskCollection     = ovfbase.DiskCollection
	StorageParam       = ovfbase.StorageParam
	StorageCollection  = ovfbase.StorageCollection
	WorkloadCollection = ovfbase.WorkloadCollection
	DetailParam        = ovfbase.DetailParam
	NameParam          = ovfbase.NameParam
)

// Routes - these are provider-specific
const (
	ProvidersRoot = Root
	ProviderRoot  = ProvidersRoot + "/:" + ProviderParam
	VMsRoot       = ProviderRoot + "/" + VMCollection
	VMRoot        = VMsRoot + "/:" + VMParam
	NetworksRoot  = ProviderRoot + "/" + NetworkCollection
	NetworkRoot   = NetworksRoot + "/:" + NetworkParam
	DisksRoot     = ProviderRoot + "/" + DiskCollection
	DiskRoot      = DisksRoot + "/:" + DiskParam
	StoragesRoot  = ProviderRoot + "/" + StorageCollection
	StorageRoot   = StoragesRoot + "/:" + StorageParam
	WorkloadsRoot = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot  = WorkloadsRoot + "/:" + VMParam
	TreeRoot      = ProviderRoot + "/tree"
	TreeVMRoot    = TreeRoot + "/vm"
)
