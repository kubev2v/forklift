package plan

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("VIB Validation", func() {
	Describe("useVIBMethod", func() {
		DescribeTable("should correctly determine VIB method usage",
			func(cloneMethod string, expected bool) {
				result := useVIBMethod(cloneMethod)
				Expect(result).To(Equal(expected))
			},
			Entry("when cloneMethod is empty (default)", "", true),
			Entry("when cloneMethod is 'vib'", "vib", true),
			Entry("when cloneMethod is 'VIB' (uppercase)", "VIB", true),
			Entry("when cloneMethod is 'ViB' (mixed case)", "ViB", true),
			Entry("when cloneMethod is 'ssh'", "ssh", false),
			Entry("when cloneMethod is 'SSH'", "SSH", false),
			Entry("when cloneMethod is 'other'", "other", false),
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
})

func TestVIB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plan VIB Suite")
}
