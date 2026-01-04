package provider

import (
	"errors"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	vsphere_offload_mocks "github.com/kubev2v/forklift/pkg/lib/vsphere_offload/mocks"
	vmware_mocks "github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"go.uber.org/mock/gomock"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("VIB Validation", func() {
	var (
		reconciler     *Reconciler
		provider       *api.Provider
		secret         *core.Secret
		scheme         *runtime.Scheme
		mockCtrl       *gomock.Controller
		mockClient     *vmware_mocks.MockClient
		mockVibEnsurer *vsphere_offload_mocks.MockVIBEnsurer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = vmware_mocks.NewMockClient(mockCtrl)
		mockVibEnsurer = vsphere_offload_mocks.NewMockVIBEnsurer(mockCtrl)

		scheme = runtime.NewScheme()
		_ = api.SchemeBuilder.AddToScheme(scheme)
		_ = core.AddToScheme(scheme)

		provider = &api.Provider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-provider",
				Namespace: "test-namespace",
			},
			Spec: api.ProviderSpec{
				Type: ptr.To(api.VSphere),
				URL:  "https://vcenter.example.com/sdk",
			},
		}

		secret = &core.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"user":     []byte("admin"),
				"password": []byte("password"),
			},
		}

		client := fakeClient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(provider, secret).
			Build()

		reconciler = &Reconciler{
			Reconciler: base.Reconciler{
				Client: client,
				Log:    logging.WithName("test"),
			},
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("validateVIBReadiness", func() {
		Context("when provider is not vSphere", func() {
			It("should return nil immediately", func() {
				provider.Spec.Type = ptr.To(api.OpenStack)
				err := reconciler.validateVIBReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when inventory is not created", func() {
			It("should return nil without validating", func() {
				provider.Status.Conditions = libcnd.Conditions{}
				err := reconciler.validateVIBReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when SSH method is enabled", func() {
			It("should delete VIB conditions and return", func() {
				provider.Spec.Settings = map[string]string{
					api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
				}
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})
				provider.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})
				provider.Status.SetCondition(libcnd.Condition{
					Type:   VIBNotReady,
					Status: libcnd.True,
				})

				err := reconciler.validateVIBReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(provider.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(provider.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("when VIB check should be skipped (cached)", func() {
			It("should stage conditions and return", func() {
				provider.Annotations = map[string]string{
					"forklift.konveyor.io/vib-last-check": "2024-01-01T00:00:00Z",
				}
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				err := reconciler.validateVIBReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when credentials are empty", func() {
			It("should set VIBNotReady condition", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				emptySecret := &core.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"user":     []byte(""),
						"password": []byte(""),
					},
				}

				err := reconciler.validateVIBReadiness(provider, emptySecret)
				Expect(err).ToNot(HaveOccurred())

				condition := provider.Status.FindCondition(VIBNotReady)
				Expect(condition).ToNot(BeNil())
				Expect(condition.Status).To(Equal(libcnd.True))
				Expect(condition.Reason).To(Equal("ProviderCredentialsInvalid"))
			})
		})

		Context("when VMware client creation fails", func() {
			It("should set VIBNotReady condition and return nil", func() {
				provider.Spec.URL = "invalid://url"
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				err := reconciler.validateVIBReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())

				condition := provider.Status.FindCondition(VIBNotReady)
				Expect(condition).ToNot(BeNil())
				Expect(condition.Status).To(Equal(libcnd.True))
				Expect(condition.Reason).To(Equal("VMwareClientFailed"))
				Expect(condition.Message).To(ContainSubstring("Failed to create VMware client"))
			})
		})

		Context("when one host passes VIB validation", func() {
			It("should delete all VIB conditions (success scenario)", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").Return(nil)

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				// When all hosts pass, both VIB conditions should be deleted
				Expect(provider.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(provider.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("when one host fails VIB validation", func() {
			It("should set VIBNotReady condition with error details", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").Return(errors.New("VIB installation failed"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				// When all hosts fail, should have VIBNotReady condition only
				Expect(provider.Status.HasCondition(VIBReady)).To(BeFalse())
				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Status).To(Equal(libcnd.True))
				Expect(vibNotReady.Reason).To(Equal("VIBNotInstalled"))
				Expect(vibNotReady.Items).To(HaveLen(1))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("host1.example.com"))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("VIB installation failed"))
			})
		})

		Context("when few hosts all pass VIB validation", func() {
			It("should delete all VIB conditions (all success scenario)", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				host1 := vmware_mocks.CreateMockHost("host1.example.com")
				host2 := vmware_mocks.CreateMockHost("host2.example.com")
				host3 := vmware_mocks.CreateMockHost("host3.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1, host2, host3}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").Return(nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host2, "/bin/vmkfstools-wrapper.vib").Return(nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host3, "/bin/vmkfstools-wrapper.vib").Return(nil)

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				// When all hosts pass, both VIB conditions should be deleted
				Expect(provider.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(provider.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("when few hosts all fail VIB validation", func() {
			It("should set VIBNotReady condition with all host errors", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				host1 := vmware_mocks.CreateMockHost("host1.example.com")
				host2 := vmware_mocks.CreateMockHost("host2.example.com")
				host3 := vmware_mocks.CreateMockHost("host3.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1, host2, host3}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").Return(errors.New("installation failed on host1"))
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host2, "/bin/vmkfstools-wrapper.vib").Return(errors.New("installation failed on host2"))
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host3, "/bin/vmkfstools-wrapper.vib").Return(errors.New("installation failed on host3"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				// When all hosts fail, should have VIBNotReady condition only
				Expect(provider.Status.HasCondition(VIBReady)).To(BeFalse())
				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Status).To(Equal(libcnd.True))
				Expect(vibNotReady.Reason).To(Equal("VIBNotInstalled"))
				Expect(vibNotReady.Items).To(HaveLen(3))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("host1.example.com"))
				Expect(vibNotReady.Items[1]).To(ContainSubstring("host2.example.com"))
				Expect(vibNotReady.Items[2]).To(ContainSubstring("host3.example.com"))
			})
		})

		Context("when few hosts have mixed VIB validation results", func() {
			It("should set both VIBReady and VIBNotReady conditions", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				host1 := vmware_mocks.CreateMockHost("host1.example.com")
				host2 := vmware_mocks.CreateMockHost("host2.example.com")
				host3 := vmware_mocks.CreateMockHost("host3.example.com")
				host4 := vmware_mocks.CreateMockHost("host4.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1, host2, host3, host4}, nil)
				// host1 and host2 pass
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").Return(nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host2, "/bin/vmkfstools-wrapper.vib").Return(nil)
				// host3 and host4 fail
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host3, "/bin/vmkfstools-wrapper.vib").Return(errors.New("installation failed on host3"))
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host4, "/bin/vmkfstools-wrapper.vib").Return(errors.New("installation failed on host4"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				// When hosts are mixed, should have both VIBReady and VIBNotReady conditions
				vibReady := provider.Status.FindCondition(VIBReady)
				Expect(vibReady).ToNot(BeNil())
				Expect(vibReady.Status).To(Equal(libcnd.True))
				Expect(vibReady.Reason).To(Equal("VIBValidated"))
				Expect(vibReady.Items).To(HaveLen(2))
				Expect(vibReady.Items[0]).To(Equal("host1.example.com"))
				Expect(vibReady.Items[1]).To(Equal("host2.example.com"))

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Status).To(Equal(libcnd.True))
				Expect(vibNotReady.Reason).To(Equal("VIBNotInstalled"))
				Expect(vibNotReady.Items).To(HaveLen(2))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("host3.example.com"))
				Expect(vibNotReady.Items[1]).To(ContainSubstring("host4.example.com"))
			})
		})

		Context("EnsureVib step-by-step failure scenarios", func() {
			BeforeEach(func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})
			})

			It("should handle VIB version check failure", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to get the VIB version from ESXi host1.example.com: connection timeout"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("failed to get the VIB version"))
			})

			It("should handle datacenter discovery failure", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to retrieve host parent: permission denied"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("failed to retrieve host parent"))
			})

			It("should handle datastore retrieval failure", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to get datastore for ESXi host1.example.com"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("failed to get datastore"))
			})

			It("should handle VIB upload failure", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to upload the VIB to ESXi host1.example.com: insufficient disk space"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("failed to upload the VIB"))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("insufficient disk space"))
			})

			It("should handle VIB installation failure", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to install the VIB on ESXi host1.example.com: VIB signature verification failed"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("failed to install the VIB"))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("signature verification failed"))
			})

			It("should handle VIB skipped by ESXi", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1}, nil)
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("VIB installation was skipped by ESXi: The host is not in maintenance mode"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items[0]).To(ContainSubstring("VIB installation was skipped"))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("maintenance mode"))
			})

			It("should handle multiple hosts with different failure types", func() {
				host1 := vmware_mocks.CreateMockHost("host1.example.com")
				host2 := vmware_mocks.CreateMockHost("host2.example.com")
				host3 := vmware_mocks.CreateMockHost("host3.example.com")

				mockClient.EXPECT().GetAllHosts(gomock.Any()).Return([]*object.HostSystem{host1, host2, host3}, nil)
				// Different failure types for different hosts
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host1, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to get datastore for ESXi host1.example.com"))
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host2, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to upload the VIB to ESXi host2.example.com: network error"))
				mockVibEnsurer.EXPECT().EnsureVib(gomock.Any(), mockClient, host3, "/bin/vmkfstools-wrapper.vib").
					Return(errors.New("failed to install the VIB on ESXi host3.example.com: permission denied"))

				err := reconciler.validateVIBWithClient(provider, mockClient, mockVibEnsurer)
				Expect(err).ToNot(HaveOccurred())

				vibNotReady := provider.Status.FindCondition(VIBNotReady)
				Expect(vibNotReady).ToNot(BeNil())
				Expect(vibNotReady.Items).To(HaveLen(3))
				Expect(vibNotReady.Items[0]).To(ContainSubstring("datastore"))
				Expect(vibNotReady.Items[1]).To(ContainSubstring("upload"))
				Expect(vibNotReady.Items[2]).To(ContainSubstring("install"))
			})
		})
	})

	Describe("validateSSHReadiness", func() {
		Context("when provider is not vSphere", func() {
			It("should return nil immediately", func() {
				provider.Spec.Type = ptr.To(api.OpenStack)
				err := reconciler.validateSSHReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when inventory is not created", func() {
			It("should return nil without validating", func() {
				provider.Status.Conditions = libcnd.Conditions{}
				err := reconciler.validateSSHReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when SSH method is not enabled", func() {
			It("should delete SSH conditions and return", func() {
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})
				provider.Status.SetCondition(libcnd.Condition{
					Type:   SSHReady,
					Status: libcnd.True,
				})
				provider.Status.SetCondition(libcnd.Condition{
					Type:   SSHNotReady,
					Status: libcnd.True,
				})

				err := reconciler.validateSSHReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())
				Expect(provider.Status.HasCondition(SSHReady)).To(BeFalse())
				Expect(provider.Status.HasCondition(SSHNotReady)).To(BeFalse())
			})
		})

		Context("when SSH method is enabled but keys don't exist", func() {
			It("should set SSHNotReady condition", func() {
				provider.Spec.Settings = map[string]string{
					api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
				}
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				err := reconciler.validateSSHReadiness(provider, secret)
				Expect(err).ToNot(HaveOccurred())

				condition := provider.Status.FindCondition(SSHNotReady)
				Expect(condition).ToNot(BeNil())
				Expect(condition.Status).To(Equal(libcnd.True))
				Expect(condition.Reason).To(Equal("SSHKeysNotFound"))
				Expect(condition.Message).To(ContainSubstring("SSH keys are being generated"))
			})
		})

		Context("when provider name generation fails", func() {
			It("should return error", func() {
				provider.Name = "" // Empty name will cause SanitizeProviderName to fail
				provider.Spec.Settings = map[string]string{
					api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
				}
				provider.Status.SetCondition(libcnd.Condition{
					Type:     InventoryCreated,
					Status:   libcnd.True,
					Category: libcnd.Required,
				})

				err := reconciler.validateSSHReadiness(provider, secret)
				Expect(err).To(HaveOccurred())
			})
		})

		// Note: Full SSH validation tests would require:
		// 1. Creating SSH key secrets
		// 2. Mocking host discovery
		// 3. Mocking SSH connectivity tests
	})
})

func TestProviderValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Validation Suite")
}
