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

// GetBPFEnabled reports whether the destination Calico install runs the BPF
// dataplane, by reading spec.bpfEnabled of the cluster-scoped
// FelixConfiguration named "default". Felix defaults to the non-BPF
// dataplane, so "not enabled" is reported when the field is false or absent,
// when no "default" FelixConfiguration exists, and when the API server does
// not know the FelixConfiguration kind at all. Any other GET failure is
// returned as an error.
func GetBPFEnabled(ctx context.Context, c client.Client) (bool, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(FelixConfigurationGVK)
	err := c.Get(ctx, client.ObjectKey{Name: felixConfigurationName}, u)
	switch {
	case err == nil:
	case meta.IsNoMatchError(err) || k8serr.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
	enabled, _, err := unstructured.NestedBool(u.Object, "spec", bpfEnabledField)
	if err != nil {
		return false, fmt.Errorf("parse spec.%s: %w", bpfEnabledField, err)
	}
	return enabled, nil
}
