package ocp

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

func TestGetNetworkNameAndNamespace(t *testing.T) {
	tests := []struct {
		name         string
		networkName  string
		vmRef        *ref.Ref
		expectedName string
		expectedNS   string
	}{
		{
			name:         "no slash in network name",
			networkName:  "network",
			vmRef:        &ref.Ref{Namespace: "vmNamespace"},
			expectedName: "network",
			expectedNS:   "vmNamespace",
		},
		{
			name:         "slash in network name",
			networkName:  "namespace/network",
			vmRef:        &ref.Ref{Namespace: "vmNamespace"},
			expectedName: "network",
			expectedNS:   "namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualName, actualNS := GetNetworkNameAndNamespace(tt.networkName, &ref.Ref{Namespace: tt.vmRef.Namespace})
			if actualName != tt.expectedName || actualNS != tt.expectedNS {
				t.Errorf("got (%s, %s), want (%s, %s)", actualName, actualNS, tt.expectedName, tt.expectedNS)
			}
		})
	}
}
