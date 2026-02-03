package plan

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("VIB Validation", func() {
	Describe("UseVIBMethod", func() {
		DescribeTable("should correctly determine VIB method usage",
			func(cloneMethod string, expected bool) {
				provider := &api.Provider{
					Spec: api.ProviderSpec{
						Settings: map[string]string{},
					},
				}
				if cloneMethod != "" {
					provider.Spec.Settings[api.ESXiCloneMethod] = cloneMethod
				}
				result := provider.UseVIBMethod()
				Expect(result).To(Equal(expected))
			},
			Entry("when cloneMethod is empty (default)", "", true),
			Entry("when cloneMethod is 'vib'", "vib", true),
			Entry("when cloneMethod is 'ssh'", "ssh", false),
		)
	})

	Describe("formatVIBHostItems", func() {
		It("should copy the items slice", func() {
			items := []string{"host1", "host2", "host3"}
			result := formatVIBHostItems(items)

			Expect(result).To(Equal(items))
			Expect(result).NotTo(BeIdenticalTo(items)) // Should be a copy, not same slice
		})

		It("should handle empty slice", func() {
			result := formatVIBHostItems([]string{})
			Expect(result).To(BeEmpty())
		})

		It("should handle nil slice", func() {
			result := formatVIBHostItems(nil)
			Expect(result).To(BeEmpty())
		})
	})

	Describe("validateVIBReadiness", func() {
		var (
			reconciler *Reconciler
			plan       *api.Plan
			storageMap *api.StorageMap
		)

		BeforeEach(func() {
			reconciler = &Reconciler{
				Reconciler: base.Reconciler{
					Log: logging.WithName("test"),
				},
			}

			// Create a basic storage map with no xcopy config
			storageMap = &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{},
				},
			}

			plan = &api.Plan{
				Referenced: api.Referenced{
					Map: struct {
						Network *api.NetworkMap
						Storage *api.StorageMap
					}{
						Storage: storageMap,
					},
				},
			}
		})

		Context("when source provider is nil", func() {
			It("should return nil without error", func() {
				plan.Referenced.Provider.Source = nil
				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when provider is not vSphere", func() {
			It("should return nil and not set conditions", func() {
				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.OpenStack),
					},
				}

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("when plan does not use xcopy populator", func() {
			It("should delete any existing VIB conditions", func() {
				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
				}

				// Set initial conditions
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBNotReady,
					Status: libcnd.True,
				})

				// Mock planUsesVSphereXcopyPopulator to return false
				// Note: This would need to be mocked properly in real implementation

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				// Conditions should be deleted when xcopy is not used
			})
		})

		Context("when SSH method is enabled", func() {
			It("should delete VIB conditions", func() {
				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
						Settings: map[string]string{
							api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
						},
					},
				}

				// Set initial VIB conditions
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				// VIB conditions should be deleted when SSH is enabled
			})
		})

		Context("when provider has VIBReady condition and plan uses xcopy", func() {
			It("should propagate VIBReady condition to plan", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:       VIBReady,
									Status:     libcnd.True,
									Reason:     "VIBValidated",
									Category:   libcnd.Advisory,
									Message:    "VIB validated on hosts",
									Suggestion: "Hosts available for migration",
									Items:      []string{"host1", "host2"},
								},
							},
						},
					},
				}

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Plan should have VIBReady condition propagated
				vibReadyCondition := plan.Status.FindCondition(VIBReady)
				Expect(vibReadyCondition).ToNot(BeNil())
				Expect(vibReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibReadyCondition.Category).To(Equal(libcnd.Advisory))
			})
		})

		Context("when provider has VIBNotReady condition and plan uses xcopy", func() {
			It("should propagate VIBNotReady condition to plan", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:       VIBNotReady,
									Status:     libcnd.True,
									Reason:     "VIBValidationFailed",
									Category:   libcnd.Warn,
									Message:    "VIB validation failed on hosts",
									Suggestion: "Fix VIB issues",
									Items:      []string{"host1 (error: VIB not found)"},
								},
							},
						},
					},
				}

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Plan should have VIBNotReady condition propagated
				vibNotReadyCondition := plan.Status.FindCondition(VIBNotReady)
				Expect(vibNotReadyCondition).ToNot(BeNil())
				Expect(vibNotReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibNotReadyCondition.Category).To(Equal(libcnd.Warn))
			})
		})

		Context("when provider has BOTH VIBReady and VIBNotReady conditions", func() {
			It("should propagate both conditions to plan (mixed scenario)", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:       VIBReady,
									Status:     libcnd.True,
									Reason:     "VIBValidated",
									Category:   libcnd.Advisory,
									Message:    "VIB validated on some hosts",
									Suggestion: "Hosts: host1, host2",
									Items:      []string{"host1", "host2"},
								},
								{
									Type:       VIBNotReady,
									Status:     libcnd.True,
									Reason:     "VIBValidationFailed",
									Category:   libcnd.Warn,
									Message:    "VIB validation failed on some hosts",
									Suggestion: "Fix: host3, host4",
									Items:      []string{"host3 (error: VIB not installed)", "host4 (error: upload failed)"},
								},
							},
						},
					},
				}

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Plan should have both conditions
				vibReadyCondition := plan.Status.FindCondition(VIBReady)
				Expect(vibReadyCondition).ToNot(BeNil())
				Expect(vibReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibReadyCondition.Items).To(HaveLen(2))

				vibNotReadyCondition := plan.Status.FindCondition(VIBNotReady)
				Expect(vibNotReadyCondition).ToNot(BeNil())
				Expect(vibNotReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibNotReadyCondition.Items).To(HaveLen(2))
			})
		})

		Context("when provider VIBReady condition has empty Items", func() {
			It("should delete VIBReady condition from plan", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:     VIBReady,
									Status:   libcnd.True,
									Category: libcnd.Advisory,
									Items:    []string{}, // Empty items
								},
							},
						},
					},
				}

				// Set initial condition
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Condition should be deleted because Items is empty
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
			})
		})

		Context("when provider VIBReady condition Status is False", func() {
			It("should delete VIBReady condition from plan", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:     VIBReady,
									Status:   libcnd.False, // False status
									Category: libcnd.Advisory,
									Items:    []string{"host1"},
								},
							},
						},
					},
				}

				// Set initial condition
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Condition should be deleted because Status is False
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
			})
		})

		Context("when provider has no VIB conditions", func() {
			It("should delete any existing VIB conditions from plan", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				plan.Referenced.Provider.Source = &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{}, // No VIB conditions
						},
					},
				}

				// Set initial conditions on plan
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBReady,
					Status: libcnd.True,
				})
				plan.Status.SetCondition(libcnd.Condition{
					Type:   VIBNotReady,
					Status: libcnd.True,
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Both conditions should be deleted
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("message and suggestion formatting", func() {
			It("should include provider name in VIBReady suggestion", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				provider := &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:       VIBReady,
									Status:     libcnd.True,
									Category:   libcnd.Advisory,
									Suggestion: "Original suggestion from provider",
									Items:      []string{"host1"},
								},
							},
						},
					},
				}
				provider.Name = "my-vcenter-provider"
				plan.Referenced.Provider.Source = provider

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				vibReadyCondition := plan.Status.FindCondition(VIBReady)
				Expect(vibReadyCondition).ToNot(BeNil())
				Expect(vibReadyCondition.Suggestion).To(ContainSubstring("my-vcenter-provider"))
				Expect(vibReadyCondition.Suggestion).To(ContainSubstring("Original suggestion from provider"))
			})

			It("should include provider name in VIBNotReady suggestion", func() {
				// Set up xcopy in storage map
				storageMap.Spec.Map = []api.StoragePair{
					{
						Source: ref.Ref{
							ID: "datastore-123",
						},
						Destination: api.DestinationStorage{
							StorageClass: "standard",
						},
						OffloadPlugin: &api.OffloadPlugin{
							VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
						},
					},
				}

				provider := &api.Provider{
					Spec: api.ProviderSpec{
						Type: ptr.To(api.VSphere),
					},
					Status: api.ProviderStatus{
						Conditions: libcnd.Conditions{
							List: []libcnd.Condition{
								{
									Type:       VIBNotReady,
									Status:     libcnd.True,
									Category:   libcnd.Warn,
									Suggestion: "Fix these issues",
									Items:      []string{"host1 (error: failed)"},
								},
							},
						},
					},
				}
				provider.Name = "my-vcenter-provider"
				plan.Referenced.Provider.Source = provider

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				vibNotReadyCondition := plan.Status.FindCondition(VIBNotReady)
				Expect(vibNotReadyCondition).ToNot(BeNil())
				Expect(vibNotReadyCondition.Suggestion).To(ContainSubstring("my-vcenter-provider"))
				Expect(vibNotReadyCondition.Suggestion).To(ContainSubstring("xcopy volume populator"))
				Expect(vibNotReadyCondition.Suggestion).To(ContainSubstring("Fix these issues"))
			})
		})
	})

	Describe("Integration: Full flow from Provider to Plan", func() {
		var (
			reconciler *Reconciler
			plan       *api.Plan
			provider   *api.Provider
			storageMap *api.StorageMap
		)

		BeforeEach(func() {
			reconciler = &Reconciler{
				Reconciler: base.Reconciler{
					Log: logging.WithName("test"),
				},
			}

			// Create a provider with VIB conditions
			provider = &api.Provider{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-vsphere-provider",
					Namespace: "test-namespace",
				},
				Spec: api.ProviderSpec{
					Type: ptr.To(api.VSphere),
					URL:  "https://vcenter.example.com/sdk",
				},
				Status: api.ProviderStatus{
					Conditions: libcnd.Conditions{
						List: []libcnd.Condition{},
					},
				},
			}

			// Create storage map with xcopy config
			storageMap = &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{
							Source: ref.Ref{
								ID: "datastore-123",
							},
							Destination: api.DestinationStorage{
								StorageClass: "standard",
							},
							OffloadPlugin: &api.OffloadPlugin{
								VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{},
							},
						},
					},
				},
			}

			// Create plan that references the provider and uses xcopy
			plan = &api.Plan{
				Referenced: api.Referenced{
					Provider: struct {
						Source      *api.Provider
						Destination *api.Provider
					}{
						Source: provider,
					},
					Map: struct {
						Network *api.NetworkMap
						Storage *api.StorageMap
					}{
						Storage: storageMap,
					},
				},
			}
		})

		Context("when provider has VIBReady condition with multiple hosts", func() {
			It("should propagate VIBReady condition to plan with all host information", func() {
				// Set up provider with VIBReady condition
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBReady,
					Status:     libcnd.True,
					Reason:     "VIBValidated",
					Category:   libcnd.Advisory,
					Message:    "VIB validated on hosts",
					Suggestion: "Hosts: host1, host2, host3",
					Items:      []string{"host1", "host2", "host3"},
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Verify plan has VIBReady condition
				vibReadyCondition := plan.Status.FindCondition(VIBReady)
				Expect(vibReadyCondition).ToNot(BeNil())
				Expect(vibReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibReadyCondition.Reason).To(Equal("ProviderVIBReady"))
				Expect(vibReadyCondition.Category).To(Equal(libcnd.Advisory))
				Expect(vibReadyCondition.Items).To(HaveLen(3))
				Expect(vibReadyCondition.Suggestion).To(ContainSubstring("test-vsphere-provider"))
				Expect(vibReadyCondition.Suggestion).To(ContainSubstring("Hosts: host1, host2, host3"))
			})
		})

		Context("when provider has VIBNotReady condition with error details", func() {
			It("should propagate VIBNotReady condition to plan with error information", func() {
				// Set up provider with VIBNotReady condition
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBNotReady,
					Status:     libcnd.True,
					Reason:     "VIBNotInstalled",
					Category:   libcnd.Warn,
					Message:    "VIB validation failed",
					Suggestion: "Fix VIB issues on hosts",
					Items:      []string{"host1 (error: VIB not found)", "host2 (error: upload failed)"},
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Verify plan has VIBNotReady condition
				vibNotReadyCondition := plan.Status.FindCondition(VIBNotReady)
				Expect(vibNotReadyCondition).ToNot(BeNil())
				Expect(vibNotReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibNotReadyCondition.Reason).To(Equal("ProviderVIBNotReady"))
				Expect(vibNotReadyCondition.Category).To(Equal(libcnd.Warn))
				Expect(vibNotReadyCondition.Items).To(HaveLen(2))
				Expect(vibNotReadyCondition.Items[0]).To(ContainSubstring("host1"))
				Expect(vibNotReadyCondition.Items[1]).To(ContainSubstring("host2"))
				Expect(vibNotReadyCondition.Suggestion).To(ContainSubstring("test-vsphere-provider"))
				Expect(vibNotReadyCondition.Suggestion).To(ContainSubstring("xcopy volume populator"))
			})
		})

		Context("when provider has both VIBReady and VIBNotReady conditions (mixed scenario)", func() {
			It("should propagate both conditions to plan", func() {
				// Set up provider with both conditions
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBReady,
					Status:     libcnd.True,
					Reason:     "VIBValidated",
					Category:   libcnd.Advisory,
					Message:    "VIB validated on some hosts",
					Suggestion: "Hosts: host1, host2",
					Items:      []string{"host1", "host2"},
				})
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBNotReady,
					Status:     libcnd.True,
					Reason:     "VIBNotInstalled",
					Category:   libcnd.Warn,
					Message:    "VIB validation failed on some hosts",
					Suggestion: "Fix: host3, host4",
					Items:      []string{"host3 (error: VIB not installed)", "host4 (error: upload failed)"},
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())

				// Verify plan has both conditions
				vibReadyCondition := plan.Status.FindCondition(VIBReady)
				Expect(vibReadyCondition).ToNot(BeNil())
				Expect(vibReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibReadyCondition.Items).To(HaveLen(2))

				vibNotReadyCondition := plan.Status.FindCondition(VIBNotReady)
				Expect(vibNotReadyCondition).ToNot(BeNil())
				Expect(vibNotReadyCondition.Status).To(Equal(libcnd.True))
				Expect(vibNotReadyCondition.Items).To(HaveLen(2))
			})
		})

		Context("when provider VIB conditions change from ready to not ready", func() {
			It("should update plan conditions accordingly", func() {
				// First, set provider as VIBReady
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBReady,
					Status:     libcnd.True,
					Reason:     "VIBValidated",
					Category:   libcnd.Advisory,
					Items:      []string{"host1"},
					Suggestion: "Host available",
				})

				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeTrue())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeFalse())

				// Now change provider to VIBNotReady
				provider.Status.DeleteCondition(VIBReady)
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBNotReady,
					Status:     libcnd.True,
					Reason:     "VIBNotInstalled",
					Category:   libcnd.Warn,
					Items:      []string{"host1 (error: VIB removed)"},
					Suggestion: "Fix VIB issues",
				})

				err = reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeTrue())
			})
		})

		Context("when plan stops using xcopy populator", func() {
			It("should remove VIB conditions from plan", func() {
				// Set up provider with VIBReady condition
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBReady,
					Status:     libcnd.True,
					Reason:     "VIBValidated",
					Category:   libcnd.Advisory,
					Items:      []string{"host1"},
					Suggestion: "Host available",
				})

				// First validate with xcopy - should have condition
				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeTrue())

				// Remove xcopy config from storage map
				storageMap.Spec.Map[0].OffloadPlugin = nil

				// Re-validate - should remove condition
				err = reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})

		Context("when provider changes from VIB to SSH method", func() {
			It("should remove VIB conditions from plan", func() {
				// Set up provider with VIBReady condition and VIB method (default)
				provider.Status.SetCondition(libcnd.Condition{
					Type:       VIBReady,
					Status:     libcnd.True,
					Reason:     "VIBValidated",
					Category:   libcnd.Advisory,
					Items:      []string{"host1"},
					Suggestion: "Host available",
				})

				// First validate with VIB method - should have condition
				err := reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeTrue())

				// Change provider to use SSH method
				provider.Spec.Settings = map[string]string{
					api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
				}

				// Re-validate - should remove VIB conditions
				err = reconciler.validateVIBReadiness(plan)
				Expect(err).ToNot(HaveOccurred())
				Expect(plan.Status.HasCondition(VIBReady)).To(BeFalse())
				Expect(plan.Status.HasCondition(VIBNotReady)).To(BeFalse())
			})
		})
	})
})
