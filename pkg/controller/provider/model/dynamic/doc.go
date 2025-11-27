// Package dynamic provides hybrid models for dynamic providers.
//
// These models store:
// - Common required fields as typed table columns (for queries and change detection)
// - Complete provider data as JSON blob (for full fidelity)
//
// This approach enables:
// - Efficient change detection on common fields
// - Event firing when detectable changes occur
// - Full data preservation without losing provider-specific fields
package dynamic
