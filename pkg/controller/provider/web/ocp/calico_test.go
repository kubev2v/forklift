package ocp

import (
	"testing"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeServerResources implements serverResources with canned answers.
type fakeServerResources struct {
	resources *meta.APIResourceList
	err       error
}

func (f *fakeServerResources) ServerResourcesForGroupVersion(gv string) (*meta.APIResourceList, error) {
	if gv != CalicoGroupVersion {
		return nil, k8serr.NewNotFound(schema.GroupResource{}, gv)
	}
	return f.resources, f.err
}

func withResources(names ...string) *meta.APIResourceList {
	list := &meta.APIResourceList{GroupVersion: CalicoGroupVersion}
	for _, name := range names {
		list.APIResources = append(list.APIResources, meta.APIResource{Name: name})
	}
	return list
}

func TestProbeCalicoCapability(t *testing.T) {
	notFound := k8serr.NewNotFound(
		schema.GroupResource{Group: "projectcalico.org"}, "v3")
	unavailable := k8serr.NewServiceUnavailable("calico-apiserver unavailable")

	tests := []struct {
		name    string
		fake    *fakeServerResources
		want    CalicoCapability
		wantErr bool
	}{
		{
			// No Calico at all: the group is absent. The correct answer,
			// not an error.
			name: "group absent",
			fake: &fakeServerResources{err: notFound},
			want: CalicoCapability{},
		},
		{
			// Calico installed, but the install does not serve the Network
			// resource.
			name: "ippools only",
			fake: &fakeServerResources{resources: withResources("ippools", "felixconfigurations")},
			want: CalicoCapability{IPPools: true},
		},
		{
			// Full capability.
			name: "ippools and networks",
			fake: &fakeServerResources{resources: withResources("ippools", "networks", "bgppeers")},
			want: CalicoCapability{IPPools: true, Networks: true},
		},
		{
			// Group served but neither probed resource present.
			name: "unrelated resources only",
			fake: &fakeServerResources{resources: withResources("felixconfigurations")},
			want: CalicoCapability{},
		},
		{
			// The aggregated calico-apiserver is registered but unhealthy:
			// present-but-broken must surface as an error, not as "absent".
			name:    "apiserver unavailable",
			fake:    &fakeServerResources{err: unavailable},
			want:    CalicoCapability{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := probeCalicoCapability(tt.fake)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("capability = %+v, want %+v", got, tt.want)
			}
		})
	}
}
