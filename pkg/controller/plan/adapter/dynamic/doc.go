// Package dynamic provides a generic adapter for dynamic inventory providers.
//
// This adapter works with ANY dynamic provider by using schema definitions
// and unstructured data access. It eliminates the need for provider-specific
// code in the controller.
//
// Key Components:
//   - Schema: Field mappings from JSON to Kubernetes resources
//   - Builder: Generic builder using unstructured data
//   - Validator: Generic validator using schema
//   - Adapter: Ties everything together
//
// Usage:
//
//	adapter := dynamic.NewAdapter(provider, context)
//	builder, _ := adapter.Builder(context)
//	dvs, _ := builder.DataVolumes(vmRef, ...)
//
// See docs/enhancements/DYNAMIC-PROVIDER-PROPOSAL.md for complete architecture.
package dynamic
