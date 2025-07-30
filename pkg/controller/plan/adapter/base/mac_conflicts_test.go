package base

import (
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	cnv "kubevirt.io/api/core/v1"
)

// MockInventoryClient implements InventoryClient for testing
type MockInventoryClient struct {
	ListFunc func(interface{}, ...webbase.Param) error
	FindFunc func(interface{}, ref.Ref) error
}

func (m *MockInventoryClient) List(obj interface{}, params ...webbase.Param) error {
	if m.ListFunc != nil {
		return m.ListFunc(obj, params...)
	}
	return nil
}

func (m *MockInventoryClient) Find(obj interface{}, vmRef ref.Ref) error {
	if m.FindFunc != nil {
		return m.FindFunc(obj, vmRef)
	}
	return nil
}

func TestExtractMACsFromInterfaces(t *testing.T) {
	tests := []struct {
		name       string
		interfaces []cnv.Interface
		expected   []string
	}{
		{
			name:       "empty interfaces",
			interfaces: []cnv.Interface{},
			expected:   []string{},
		},
		{
			name: "single interface",
			interfaces: []cnv.Interface{
				{MacAddress: "aa:bb:cc:dd:ee:ff"},
			},
			expected: []string{"aa:bb:cc:dd:ee:ff"},
		},
		{
			name: "multiple interfaces",
			interfaces: []cnv.Interface{
				{MacAddress: "aa:bb:cc:dd:ee:ff"},
				{MacAddress: "11:22:33:44:55:66"},
				{MacAddress: "77:88:99:aa:bb:cc"},
			},
			expected: []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66", "77:88:99:aa:bb:cc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMACsFromInterfaces(tt.interfaces)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d MACs, got %d", len(tt.expected), len(result))
				return
			}
			for i, mac := range result {
				if mac != tt.expected[i] {
					t.Errorf("Expected MAC[%d] = %s, got %s", i, tt.expected[i], mac)
				}
			}
		})
	}
}

func TestGetDestinationVMsFromInventory(t *testing.T) {
	tests := []struct {
		name          string
		mockListFunc  func(interface{}, ...webbase.Param) error
		expectedError bool
		expectedVMs   []DestinationVM
	}{
		{
			name: "successful_list_with_multiple_VMs",
			mockListFunc: func(obj interface{}, params ...webbase.Param) error {
				list := obj.(*[]ocp.VM)
				*list = []ocp.VM{
					{
						Resource: ocp.Resource{
							Namespace: "test-ns",
							Name:      "vm1",
						},
						Object: cnv.VirtualMachine{
							Spec: cnv.VirtualMachineSpec{
								Template: &cnv.VirtualMachineInstanceTemplateSpec{
									Spec: cnv.VirtualMachineInstanceSpec{
										Domain: cnv.DomainSpec{
											Devices: cnv.Devices{
												Interfaces: []cnv.Interface{
													{MacAddress: "aa:bb:cc:dd:ee:ff"},
													{MacAddress: "11:22:33:44:55:66"},
												},
											},
										},
									},
								},
							},
						},
					},
				}
				return nil
			},
			expectedError: false,
			expectedVMs: []DestinationVM{
				{
					Namespace: "test-ns",
					Name:      "vm1",
					MACs:      []string{"aa:bb:cc:dd:ee:ff", "11:22:33:44:55:66"},
				},
			},
		},
		{
			name: "empty_list",
			mockListFunc: func(obj interface{}, params ...webbase.Param) error {
				list := obj.(*[]ocp.VM)
				*list = []ocp.VM{}
				return nil
			},
			expectedError: false,
			expectedVMs:   []DestinationVM{},
		},
		{
			name: "list_error",
			mockListFunc: func(obj interface{}, params ...webbase.Param) error {
				return fmt.Errorf("inventory error")
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockInventoryClient{
				ListFunc: tt.mockListFunc,
			}

			result, err := GetDestinationVMsFromInventory(mockClient)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expectedVMs) {
				t.Errorf("Expected %d VMs, got %d", len(tt.expectedVMs), len(result))
				return
			}

			for i, expected := range tt.expectedVMs {
				actual := result[i]
				if actual.Namespace != expected.Namespace || actual.Name != expected.Name {
					t.Errorf("VM[%d] mismatch: expected %s/%s, got %s/%s",
						i, expected.Namespace, expected.Name, actual.Namespace, actual.Name)
				}
				if len(actual.MACs) != len(expected.MACs) {
					t.Errorf("VM[%d] MAC count mismatch: expected %d, got %d",
						i, len(expected.MACs), len(actual.MACs))
				}
			}
		})
	}
}

func TestGetDestinationVMsFromInventoryPassesParams(t *testing.T) {
	mockClient := &MockInventoryClient{
		ListFunc: func(obj interface{}, params ...webbase.Param) error {
			// Verify that parameters are passed through
			if len(params) != 2 {
				return fmt.Errorf("expected 2 params, got %d", len(params))
			}
			return nil
		},
	}

	// Test that parameters are passed through
	param1 := webbase.Param{Key: "namespace", Value: "test"}
	param2 := webbase.Param{Key: "label", Value: "app=test"}

	_, err := GetDestinationVMsFromInventory(mockClient, param1, param2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}
