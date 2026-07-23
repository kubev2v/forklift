package calico

import (
	"context"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FelixConfigurationGVK is the GroupVersionKind of projectcalico.org/v3
// FelixConfiguration.
var FelixConfigurationGVK = schema.GroupVersionKind{
	Group:   "projectcalico.org",
	Version: "v3",
	Kind:    "FelixConfiguration",
}

// felixConfigurationName is the name of the cluster-wide FelixConfiguration.
const felixConfigurationName = "default"

// bpfEnabledField is the FelixConfiguration spec field that switches Felix to
// the BPF dataplane. This is the single place the field name appears — if the
// canonical name turns out to differ, correcting this constant is the fix.
const bpfEnabledField = "bpfEnabled"

// nftablesModeField is the FelixConfiguration spec field that switches Felix
// to the nftables dataplane ("Disabled" | "Auto" | "Enabled"; Felix treats an
// absent field as Disabled). Single-constant field name, like bpfEnabledField.
const nftablesModeField = "nftablesMode"

// routeTableRangesField is the FelixConfiguration spec field listing the
// kernel route table ranges Calico claims for its own route programming.
// Single-constant field name, like bpfEnabledField.
const routeTableRangesField = "routeTableRanges"

// NftablesModeEnabled is the spec.nftablesMode value that runs Felix on the
// nftables dataplane — the only dataplane VRF networking supports.
const NftablesModeEnabled = "Enabled"

// RouteTableRange is a parsed entry of FelixConfiguration
// spec.routeTableRanges.
type RouteTableRange struct {
	Min int64
	Max int64
}

// FelixConfig holds the dataplane facts the plan validators read from the
// cluster-wide "default" FelixConfiguration.
type FelixConfig struct {
	// BPFEnabled reports spec.bpfEnabled (false when absent — Felix
	// defaults to the non-BPF dataplane).
	BPFEnabled bool
	// NftablesMode reports spec.nftablesMode verbatim; empty when absent
	// (Felix then treats the mode as Disabled).
	NftablesMode string
	// RouteTableRanges reports spec.routeTableRanges; nil when the field
	// is absent — callers must not substitute Felix's built-in defaults.
	RouteTableRanges []RouteTableRange
}

// GetFelixConfig reads the cluster-scoped FelixConfiguration named "default"
// and returns its dataplane facts. When no "default" FelixConfiguration
// exists, or the API server does not know the FelixConfiguration kind at
// all, Felix runs on its built-in defaults and the zero-value FelixConfig is
// returned. Any other GET failure is returned as an error.
func GetFelixConfig(ctx context.Context, c client.Client) (*FelixConfig, error) {
	fc := &FelixConfig{}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(FelixConfigurationGVK)
	err := c.Get(ctx, client.ObjectKey{Name: felixConfigurationName}, u)
	switch {
	case err == nil:
	case meta.IsNoMatchError(err) || k8serr.IsNotFound(err):
		return fc, nil
	default:
		return nil, err
	}
	fc.BPFEnabled, _, err = unstructured.NestedBool(u.Object, "spec", bpfEnabledField)
	if err != nil {
		return nil, fmt.Errorf("parse spec.%s: %w", bpfEnabledField, err)
	}
	fc.NftablesMode, _, err = unstructured.NestedString(u.Object, "spec", nftablesModeField)
	if err != nil {
		return nil, fmt.Errorf("parse spec.%s: %w", nftablesModeField, err)
	}
	rangesRaw, found, err := unstructured.NestedSlice(u.Object, "spec", routeTableRangesField)
	if err != nil {
		return nil, fmt.Errorf("parse spec.%s: %w", routeTableRangesField, err)
	}
	if found {
		fc.RouteTableRanges = make([]RouteTableRange, 0, len(rangesRaw))
		for i, r := range rangesRaw {
			m, ok := r.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("spec.%s[%d]: not an object", routeTableRangesField, i)
			}
			var rng RouteTableRange
			if rng.Min, ok = asInt64(m["min"]); !ok {
				return nil, fmt.Errorf("spec.%s[%d].min: not an integer (%v)", routeTableRangesField, i, m["min"])
			}
			if rng.Max, ok = asInt64(m["max"]); !ok {
				return nil, fmt.Errorf("spec.%s[%d].max: not an integer (%v)", routeTableRangesField, i, m["max"])
			}
			fc.RouteTableRanges = append(fc.RouteTableRanges, rng)
		}
	}
	return fc, nil
}

// GetBPFEnabled reports whether the destination Calico install runs the BPF
// dataplane, by reading spec.bpfEnabled via GetFelixConfig. Felix defaults
// to the non-BPF dataplane, so "not enabled" is reported when the field,
// the "default" FelixConfiguration, or the FelixConfiguration kind itself
// is absent. Any other GET failure is returned as an error.
func GetBPFEnabled(ctx context.Context, c client.Client) (bool, error) {
	fc, err := GetFelixConfig(ctx, c)
	if err != nil {
		return false, err
	}
	return fc.BPFEnabled, nil
}
