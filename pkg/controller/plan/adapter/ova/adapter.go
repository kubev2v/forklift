// Package ova provides the OVA plan adapter.
// It uses the shared ovfbase logic for OVF-based providers.
package ova

import (
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/ovfbase"
)

// Type aliases for backward compatibility.
// OVA and HyperV share the same adapter logic since both use OVF format.
type (
	Adapter           = ovfbase.Adapter
	Builder           = ovfbase.Builder
	Client            = ovfbase.Client
	Validator         = ovfbase.Validator
	DestinationClient = ovfbase.DestinationClient
)
