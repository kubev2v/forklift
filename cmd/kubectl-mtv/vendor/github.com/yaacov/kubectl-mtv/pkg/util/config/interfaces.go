package config

import "k8s.io/cli-runtime/pkg/genericclioptions"

// InventoryConfigGetter defines the interface for getting inventory service configuration.
// This interface is shared across multiple commands that need to access the MTV inventory service.
type InventoryConfigGetter interface {
	GetInventoryURL() string
	GetInventoryInsecureSkipTLS() bool
}

// InventoryConfigWithKubeFlags extends InventoryConfigGetter with KubeConfigFlags access.
// This is useful for commands that need inventory configuration and Kubernetes client access.
type InventoryConfigWithKubeFlags interface {
	InventoryConfigGetter
	GetKubeConfigFlags() *genericclioptions.ConfigFlags
}

// InventoryConfigWithVerbosity extends InventoryConfigGetter with verbosity control.
// This is useful for commands that need inventory configuration and logging control.
type InventoryConfigWithVerbosity interface {
	InventoryConfigGetter
	GetVerbosity() int
}

// GlobalConfigGetter defines the full interface for accessing global configuration.
// This extends InventoryConfigGetter with additional configuration methods used by various commands.
type GlobalConfigGetter interface {
	InventoryConfigGetter
	GetVerbosity() int
	GetAllNamespaces() bool
	GetUseUTC() bool
	GetKubeConfigFlags() *genericclioptions.ConfigFlags
}
