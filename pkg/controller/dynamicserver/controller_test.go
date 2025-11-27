package dynamicserver

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestSyncServerSpec(t *testing.T) {
	log := logging.WithName("test")

	tests := []struct {
		name            string
		server          *api.DynamicProviderServer
		dynamicProvider *api.DynamicProvider
		provider        *api.Provider
		expectedUpdated bool
		verifyFunc      func(*testing.T, *api.DynamicProviderServer)
	}{
		{
			name: "Copy all fields from DynamicProvider and Provider",
			server: &api.DynamicProviderServer{
				Spec: api.DynamicProviderServerSpec{},
			},
			dynamicProvider: &api.DynamicProvider{
				Spec: api.DynamicProviderSpec{
					Image: "test-image:latest",
					ImagePullPolicy: func() *v1.PullPolicy {
						p := v1.PullAlways
						return &p
					}(),
					ImagePullSecrets: []v1.LocalObjectReference{
						{Name: "my-secret"},
					},
					Port:            ptr.To(int32(9090)),
					RefreshInterval: ptr.To(int32(600)),
					Resources: &v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("256Mi"),
							v1.ResourceCPU:    resource.MustParse("100m"),
						},
					},
					Storages: []api.StorageSpec{
						{
							Name:      "cache",
							Size:      "10Gi",
							MountPath: "/cache",
						},
					},
					Env: []v1.EnvVar{
						{Name: "TEST_ENV", Value: "test-value"},
					},
				},
			},
			provider: &api.Provider{
				Spec: api.ProviderSpec{
					Volumes: []api.ProviderVolume{
						{
							Name:      "data",
							MountPath: "/data",
							VolumeSource: v1.VolumeSource{
								NFS: &v1.NFSVolumeSource{
									Server: "nfs.example.com",
									Path:   "/exports/data",
								},
							},
						},
					},
					ServerNodeSelector: map[string]string{
						"disktype": "ssd",
					},
					ServerAffinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/os",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"linux"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedUpdated: true,
			verifyFunc: func(t *testing.T, server *api.DynamicProviderServer) {
				if server.Spec.Image != "test-image:latest" {
					t.Errorf("Expected Image to be 'test-image:latest', got '%s'", server.Spec.Image)
				}
				if server.Spec.ImagePullPolicy == nil || *server.Spec.ImagePullPolicy != v1.PullAlways {
					t.Errorf("Expected ImagePullPolicy to be PullAlways")
				}
				if len(server.Spec.ImagePullSecrets) != 1 || server.Spec.ImagePullSecrets[0].Name != "my-secret" {
					t.Errorf("Expected ImagePullSecrets to contain 'my-secret'")
				}
				if server.Spec.Port == nil || *server.Spec.Port != 9090 {
					t.Errorf("Expected Port to be 9090")
				}
				if server.Spec.RefreshInterval == nil || *server.Spec.RefreshInterval != 600 {
					t.Errorf("Expected RefreshInterval to be 600")
				}
				if server.Spec.Resources == nil {
					t.Errorf("Expected Resources to be copied")
				}
				if len(server.Spec.Storages) != 1 {
					t.Errorf("Expected 1 Storage, got %d", len(server.Spec.Storages))
				}
				if len(server.Spec.Env) != 1 {
					t.Errorf("Expected 1 Env var, got %d", len(server.Spec.Env))
				}
				if len(server.Spec.Volumes) != 1 {
					t.Errorf("Expected 1 Volume, got %d", len(server.Spec.Volumes))
				}
				if len(server.Spec.NodeSelector) != 1 {
					t.Errorf("Expected 1 NodeSelector entry, got %d", len(server.Spec.NodeSelector))
				}
				if server.Spec.Affinity == nil {
					t.Errorf("Expected Affinity to be copied")
				}
			},
		},
		{
			name: "Don't override existing fields",
			server: &api.DynamicProviderServer{
				Spec: api.DynamicProviderServerSpec{
					Image: "custom-image:latest",
					Resources: &v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
					NodeSelector: map[string]string{
						"custom": "selector",
					},
				},
			},
			dynamicProvider: &api.DynamicProvider{
				Spec: api.DynamicProviderSpec{
					Image: "default-image:latest",
					Resources: &v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			provider: &api.Provider{
				Spec: api.ProviderSpec{
					ServerNodeSelector: map[string]string{
						"default": "selector",
					},
				},
			},
			expectedUpdated: false,
			verifyFunc: func(t *testing.T, server *api.DynamicProviderServer) {
				if server.Spec.Image != "custom-image:latest" {
					t.Errorf("Expected Image to remain 'custom-image:latest', got '%s'", server.Spec.Image)
				}
				if server.Spec.Resources.Requests.Memory().String() != "512Mi" {
					t.Errorf("Expected Resources to remain custom value")
				}
				if len(server.Spec.NodeSelector) != 1 || server.Spec.NodeSelector["custom"] != "selector" {
					t.Errorf("Expected NodeSelector to remain custom value")
				}
			},
		},
		{
			name: "Partial copy - only missing fields",
			server: &api.DynamicProviderServer{
				Spec: api.DynamicProviderServerSpec{
					Image: "existing-image:latest",
				},
			},
			dynamicProvider: &api.DynamicProvider{
				Spec: api.DynamicProviderSpec{
					Image: "default-image:latest",
					Port:  ptr.To(int32(8080)),
					Resources: &v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
			provider: &api.Provider{
				Spec: api.ProviderSpec{
					ServerNodeSelector: map[string]string{
						"disktype": "ssd",
					},
				},
			},
			expectedUpdated: true,
			verifyFunc: func(t *testing.T, server *api.DynamicProviderServer) {
				// Image should not be overridden
				if server.Spec.Image != "existing-image:latest" {
					t.Errorf("Expected Image to remain 'existing-image:latest', got '%s'", server.Spec.Image)
				}
				// Port should be copied
				if server.Spec.Port == nil || *server.Spec.Port != 8080 {
					t.Errorf("Expected Port to be copied as 8080")
				}
				// Resources should be copied
				if server.Spec.Resources == nil {
					t.Errorf("Expected Resources to be copied")
				}
				// NodeSelector should be copied
				if len(server.Spec.NodeSelector) != 1 || server.Spec.NodeSelector["disktype"] != "ssd" {
					t.Errorf("Expected NodeSelector to be copied")
				}
			},
		},
		{
			name: "Empty sources - no changes",
			server: &api.DynamicProviderServer{
				Spec: api.DynamicProviderServerSpec{},
			},
			dynamicProvider: &api.DynamicProvider{
				Spec: api.DynamicProviderSpec{},
			},
			provider: &api.Provider{
				Spec: api.ProviderSpec{},
			},
			expectedUpdated: false,
			verifyFunc: func(t *testing.T, server *api.DynamicProviderServer) {
				// Nothing should be set
				if server.Spec.Image != "" {
					t.Errorf("Expected Image to remain empty")
				}
				if server.Spec.Resources != nil {
					t.Errorf("Expected Resources to remain nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &Reconciler{}
			reconciler.Log = log

			updated := reconciler.syncServerSpec(tt.server, tt.dynamicProvider, tt.provider)

			if updated != tt.expectedUpdated {
				t.Errorf("Expected updated=%v, got %v", tt.expectedUpdated, updated)
			}

			if tt.verifyFunc != nil {
				tt.verifyFunc(t, tt.server)
			}
		})
	}
}

func TestSyncServerSpec_EnvMerge(t *testing.T) {
	log := logging.WithName("test")
	reconciler := &Reconciler{}
	reconciler.Log = log

	server := &api.DynamicProviderServer{
		Spec: api.DynamicProviderServerSpec{},
	}

	dynamicProvider := &api.DynamicProvider{
		Spec: api.DynamicProviderSpec{
			Env: []v1.EnvVar{
				{Name: "ENV1", Value: "value1"},
				{Name: "ENV2", Value: "value2"},
			},
		},
	}

	provider := &api.Provider{
		Spec: api.ProviderSpec{},
	}

	updated := reconciler.syncServerSpec(server, dynamicProvider, provider)

	if !updated {
		t.Errorf("Expected update to occur")
	}

	if len(server.Spec.Env) != 2 {
		t.Errorf("Expected 2 env vars, got %d", len(server.Spec.Env))
	}

	// Verify env vars were deep copied
	server.Spec.Env[0].Value = "modified"
	if dynamicProvider.Spec.Env[0].Value == "modified" {
		t.Errorf("Env vars should be deep copied, not shared")
	}
}

func TestSyncServerSpec_StoragesCopy(t *testing.T) {
	log := logging.WithName("test")
	reconciler := &Reconciler{}
	reconciler.Log = log

	server := &api.DynamicProviderServer{
		Spec: api.DynamicProviderServerSpec{},
	}

	dynamicProvider := &api.DynamicProvider{
		Spec: api.DynamicProviderSpec{
			Storages: []api.StorageSpec{
				{
					Name:      "cache",
					Size:      "10Gi",
					MountPath: "/cache",
				},
				{
					Name:      "workspace",
					Size:      "50Gi",
					MountPath: "/workspace",
				},
			},
		},
	}

	provider := &api.Provider{
		Spec: api.ProviderSpec{},
	}

	updated := reconciler.syncServerSpec(server, dynamicProvider, provider)

	if !updated {
		t.Errorf("Expected update to occur")
	}

	if len(server.Spec.Storages) != 2 {
		t.Errorf("Expected 2 storages, got %d", len(server.Spec.Storages))
	}

	// Verify first storage
	if server.Spec.Storages[0].Name != "cache" || server.Spec.Storages[0].Size != "10Gi" {
		t.Errorf("Storage not copied correctly")
	}
}
