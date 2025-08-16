package ocp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	cnv "kubevirt.io/api/core/v1"
	export "kubevirt.io/api/export/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// createTestScheme creates a runtime scheme with KubeVirt types registered
func createTestScheme() *runtime.Scheme {
	testScheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(testScheme))
	utilruntime.Must(cnv.AddToScheme(testScheme))
	return testScheme
}

// createTestBuilder creates a Builder for testing
func createTestBuilder() *Builder {
	return &Builder{
		Context: &plancontext.Context{
			Log: logging.WithName("test-builder"),
		},
	}
}

func TestFindVMInManifestItems(t *testing.T) {
	t.Parallel()

	// Helper to create a test VM
	createTestVM := func(name, namespace, macAddress string) *cnv.VirtualMachine {
		return &cnv.VirtualMachine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kubevirt.io/v1",
				Kind:       "VirtualMachine",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: cnv.VirtualMachineSpec{
				Template: &cnv.VirtualMachineInstanceTemplateSpec{
					Spec: cnv.VirtualMachineInstanceSpec{
						Domain: cnv.DomainSpec{
							Devices: cnv.Devices{
								Interfaces: []cnv.Interface{
									{
										Name:       "net-0",
										MacAddress: macAddress,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	// Helper to create a test ConfigMap
	createTestConfigMap := func() *core.ConfigMap {
		return &core.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-config",
				Namespace: "test-namespace",
			},
			Data: map[string]string{
				"key": "value",
			},
		}
	}

	tests := []struct {
		name        string
		setupItems  func() []runtime.RawExtension
		expectError bool
		expectVM    bool
		vmName      string
		vmNamespace string
		vmMAC       string
	}{
		{
			name: "finds VM in manifest items",
			setupItems: func() []runtime.RawExtension {
				vm := createTestVM("test-vm", "test-namespace", "00:50:56:be:b2:36")
				vmBytes, _ := json.Marshal(vm)
				return []runtime.RawExtension{{Raw: vmBytes}}
			},
			expectError: false,
			expectVM:    true,
			vmName:      "test-vm",
			vmNamespace: "test-namespace",
			vmMAC:       "00:50:56:be:b2:36",
		},
		{
			name: "finds VM with different name and MAC",
			setupItems: func() []runtime.RawExtension {
				vm := createTestVM("core-test-vm", "core-test-namespace", "00:50:56:be:b2:37")
				vmBytes, _ := json.Marshal(vm)
				return []runtime.RawExtension{{Raw: vmBytes}}
			},
			expectError: false,
			expectVM:    true,
			vmName:      "core-test-vm",
			vmNamespace: "core-test-namespace",
			vmMAC:       "00:50:56:be:b2:37",
		},
		{
			name: "returns error with empty items",
			setupItems: func() []runtime.RawExtension {
				return []runtime.RawExtension{}
			},
			expectError: true,
			expectVM:    false,
		},
		{
			name: "returns error when no VM in items",
			setupItems: func() []runtime.RawExtension {
				configMap := createTestConfigMap()
				configMapBytes, _ := json.Marshal(configMap)
				return []runtime.RawExtension{{Raw: configMapBytes}}
			},
			expectError: true,
			expectVM:    false,
		},
		{
			name: "skips invalid item and finds VM",
			setupItems: func() []runtime.RawExtension {
				invalid := []byte(`{"invalid": json"}`)
				vm := createTestVM("resilient-vm", "test-namespace", "00:50:56:be:b2:99")
				vmBytes, _ := json.Marshal(vm)
				return []runtime.RawExtension{{Raw: invalid}, {Raw: vmBytes}}
			},
			expectError: false,
			expectVM:    true,
			vmName:      "resilient-vm",
			vmNamespace: "test-namespace",
			vmMAC:       "00:50:56:be:b2:99",
		},
		{
			name: "returns error when only invalid items",
			setupItems: func() []runtime.RawExtension {
				return []runtime.RawExtension{{Raw: []byte(`{"invalid": json"}`)}}
			},
			expectError: true,
			expectVM:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			items := tt.setupItems()
			builder := createTestBuilder()
			testScheme := createTestScheme()
			decode := serializer.NewCodecFactory(testScheme).UniversalDeserializer().Decode

			// Execute
			foundVM, err := builder.findVMInManifestItems(items, decode)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Fatalf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify VM expectation
			if tt.expectVM && foundVM == nil {
				t.Fatalf("Expected to find VM but got nil")
			}
			if !tt.expectVM && foundVM != nil {
				t.Errorf("Expected nil VM but got %v", foundVM)
			}

			// Verify VM details if expected
			if tt.expectVM && foundVM != nil {
				if foundVM.Name != tt.vmName {
					t.Errorf("Expected VM name '%s', got '%s'", tt.vmName, foundVM.Name)
				}
				if foundVM.Namespace != tt.vmNamespace {
					t.Errorf("Expected VM namespace '%s', got '%s'", tt.vmNamespace, foundVM.Namespace)
				}
				if len(foundVM.Spec.Template.Spec.Domain.Devices.Interfaces) > 0 {
					iface := foundVM.Spec.Template.Spec.Domain.Devices.Interfaces[0]
					if iface.MacAddress != tt.vmMAC {
						t.Errorf("Expected MAC address '%s', got '%s'", tt.vmMAC, iface.MacAddress)
					}
				}
			}
		})
	}
}

func TestGetSourceVmFromDefinition_HTTPFormats(t *testing.T) {
	t.Parallel()

	// Helper to create test VM
	createTestVM := func(name, namespace, mac string) *cnv.VirtualMachine {
		return &cnv.VirtualMachine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kubevirt.io/v1",
				Kind:       "VirtualMachine",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: cnv.VirtualMachineSpec{
				Template: &cnv.VirtualMachineInstanceTemplateSpec{
					Spec: cnv.VirtualMachineInstanceSpec{
						Domain: cnv.DomainSpec{
							Devices: cnv.Devices{
								Interfaces: []cnv.Interface{
									{
										Name:       "default",
										MacAddress: mac,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name           string
		setupServer    func() (string, func())
		expectError    bool
		expectedVMName string
		expectedMAC    string
	}{
		{
			name: "fetches VM from single VirtualMachine manifest",
			setupServer: func() (string, func()) {
				vm := createTestVM("single-vm", "test-namespace", "00:50:56:be:b2:01")
				vmBytes, _ := json.Marshal(vm)

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-kubevirt-export-token") == "" {
						http.Error(w, "missing x-kubevirt-export-token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(vmBytes); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError:    false,
			expectedVMName: "single-vm",
			expectedMAC:    "00:50:56:be:b2:01",
		},
		{
			name: "fetches VM from metav1.List manifest",
			setupServer: func() (string, func()) {
				vm := createTestVM("list-vm", "test-namespace", "00:50:56:be:b2:02")
				vmBytes, _ := json.Marshal(vm)

				list := &metav1.List{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "List",
					},
					Items: []runtime.RawExtension{
						{Raw: vmBytes},
					},
				}
				listBytes, _ := json.Marshal(list)

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-kubevirt-export-token") == "" {
						http.Error(w, "missing x-kubevirt-export-token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(listBytes); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError:    false,
			expectedVMName: "list-vm",
			expectedMAC:    "00:50:56:be:b2:02",
		},
		{
			name: "fetches VM from core.List manifest",
			setupServer: func() (string, func()) {
				vm := createTestVM("core-list-vm", "test-namespace", "00:50:56:be:b2:03")
				vmBytes, _ := json.Marshal(vm)

				list := &core.List{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "List",
					},
					Items: []runtime.RawExtension{
						{Raw: vmBytes},
					},
				}
				listBytes, _ := json.Marshal(list)

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-kubevirt-export-token") == "" {
						http.Error(w, "missing x-kubevirt-export-token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(listBytes); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError:    false,
			expectedVMName: "core-list-vm",
			expectedMAC:    "00:50:56:be:b2:03",
		},
		{
			name: "handles non-200 HTTP response",
			setupServer: func() (string, func()) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					if _, err := w.Write([]byte("Not Found")); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError: true,
		},
		{
			name: "handles manifest with no VM",
			setupServer: func() (string, func()) {
				configMap := &core.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ConfigMap",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-config",
					},
				}
				configBytes, _ := json.Marshal(configMap)

				list := &metav1.List{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "List",
					},
					Items: []runtime.RawExtension{
						{Raw: configBytes},
					},
				}
				listBytes, _ := json.Marshal(list)

				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-kubevirt-export-token") == "" {
						http.Error(w, "missing x-kubevirt-export-token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(listBytes); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError: true,
		},
		{
			name: "handles invalid JSON response",
			setupServer: func() (string, func()) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-kubevirt-export-token") == "" {
						http.Error(w, "missing x-kubevirt-export-token", http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte(`{"invalid": json"}`)); err != nil {
						http.Error(w, "Failed to write response", http.StatusInternalServerError)
					}
				}))

				return server.URL, server.Close
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test server
			serverURL, cleanup := tt.setupServer()
			defer cleanup()

			// Create VirtualMachineExport with manifest URL
			tokenSecretName := "test-token-secret"
			vmExport := &export.VirtualMachineExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-export",
					Namespace: "test-namespace",
				},
				Status: &export.VirtualMachineExportStatus{
					TokenSecretRef: &tokenSecretName,
					Links: &export.VirtualMachineExportLinks{
						External: &export.VirtualMachineExportLink{
							Manifests: []export.VirtualMachineExportManifest{
								{
									Type: export.AllManifests,
									Url:  serverURL,
								},
							},
							Cert: "", // Empty cert for test
						},
					},
				},
			}

			// Create mock token secret
			tokenSecret := &core.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tokenSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"token": []byte("test-token"),
				},
			}

			// Create builder with mock source client
			testScheme := createTestScheme()
			fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(tokenSecret).Build()
			builder := &Builder{
				Context: &plancontext.Context{
					Log: logging.WithName("test-builder"),
				},
				sourceClient: fakeClient,
			}
			vm, err := builder.getSourceVmFromDefinition(vmExport)

			// Verify expectations
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if vm == nil {
				t.Fatalf("Expected VM but got nil")
			}

			// Verify VM details
			if vm.Name != tt.expectedVMName {
				t.Errorf("Expected VM name '%s', got '%s'", tt.expectedVMName, vm.Name)
			}

			if len(vm.Spec.Template.Spec.Domain.Devices.Interfaces) > 0 {
				iface := vm.Spec.Template.Spec.Domain.Devices.Interfaces[0]
				if iface.MacAddress != tt.expectedMAC {
					t.Errorf("Expected MAC address '%s', got '%s'", tt.expectedMAC, iface.MacAddress)
				}
			}
		})
	}
}

func TestGetSourceVmFromDefinition_NoAllManifests(t *testing.T) {
	t.Parallel()

	// Test case where VirtualMachineExport has no AllManifests type
	tokenSecretName := "test-token-secret"
	vmExport := &export.VirtualMachineExport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-export",
			Namespace: "test-namespace",
		},
		Status: &export.VirtualMachineExportStatus{
			TokenSecretRef: &tokenSecretName,
			Links: &export.VirtualMachineExportLinks{
				External: &export.VirtualMachineExportLink{
					Manifests: []export.VirtualMachineExportManifest{
						{
							Type: export.AuthHeader, // Wrong type (should be AllManifests)
							Url:  "http://example.com/manifest",
						},
					},
				},
			},
		},
	}

	// Create mock token secret
	tokenSecret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tokenSecretName,
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"token": []byte("test-token"),
		},
	}

	// Create builder with mock source client
	testScheme := createTestScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(tokenSecret).Build()
	builder := &Builder{
		Context: &plancontext.Context{
			Log: logging.WithName("test-builder"),
		},
		sourceClient: fakeClient,
	}
	vm, err := builder.getSourceVmFromDefinition(vmExport)

	if err == nil {
		t.Fatalf("Expected error for missing AllManifests but got nil")
	}

	if vm != nil {
		t.Fatalf("Expected nil VM but got %v", vm)
	}

	// Verify error message is descriptive
	if !strings.Contains(err.Error(), "manifest") {
		t.Errorf("Expected error to mention manifest, got: %v", err)
	}
}

func TestBuilder_ErrorWrapping(t *testing.T) {
	t.Parallel()

	// Test that errors are properly wrapped and descriptive
	builder := createTestBuilder()

	// Test case: invalid manifest processing should return descriptive error
	// This simulates getSourceVmFromDefinition when no VM is found

	// Create items with no VM
	items := []runtime.RawExtension{
		{
			Raw: []byte(`{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "test"}}`),
		},
	}

	testScheme := createTestScheme()
	decode := serializer.NewCodecFactory(testScheme).UniversalDeserializer().Decode
	foundVM, err := builder.findVMInManifestItems(items, decode)

	if err == nil {
		t.Fatalf("Expected error when no VM found, got nil")
	}
	if foundVM != nil {
		t.Errorf("Expected nil when no VM found, got %v", foundVM)
	}

	// In the actual getSourceVmFromDefinition, this would result in:
	// return nil, liberr.New("failed to find vm in manifest")
	// We verify the helper function behaves correctly
}

// NOTE: Single VM manifest handling is tested implicitly through the switch statement tests
// The case *cnv.VirtualMachine branch in getSourceVmFromDefinition handles this scenario
