package plan

import (
	"strconv"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	vspheremodel "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	discovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	sourceNamespace  = "source-namespace"
	destNamespace    = "destination-namespace"
	testNamespace    = "test-namespace"
	sourceName       = "source"
	destName         = "destination"
	sourceSecretName = "source-secret"
	testPlanName     = "test-plan"
	tokenKey         = "token"
	tokenValue       = "token"
	insecureSkipKey  = "inscureSkipVerify"
)

var (
	planValidationLog = logging.WithName("planValidation")
)

var _ = ginkgo.Describe("Plan Validations", func() {
	var (
		fakeClientSet *fake.Clientset
		reconciler    *Reconciler
	)

	ginkgo.BeforeEach(func() {
		reconciler = &Reconciler{
			base.Reconciler{},
			nil,
		}
		fakeClientSet = fake.NewSimpleClientset()
	})

	ginkgo.Describe("validateOCPVersion", func() {
		ginkgo.DescribeTable("should validate OpenShift version correctly",
			func(major, minor string, shouldError bool) {
				fakeDiscovery, ok := fakeClientSet.Discovery().(*discovery.FakeDiscovery)
				gomega.Expect(ok).To(gomega.BeTrue())
				fakeDiscovery.FakedServerVersion = &version.Info{
					Major: major, Minor: minor,
				}

				err := reconciler.checkOCPVersion(fakeClientSet)

				if shouldError {
					gomega.Expect(err).To(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}
			},

			// Directly declare entries here
			ginkgo.Entry("when the OpenShift version is supported", "1", "26", false),
			ginkgo.Entry("when the OpenShift version is not supported", "1", "25", true),
		)
	})

	ginkgo.Describe("validate", func() {
		ginkgo.It("Should setup secret when source is not local cluster", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).NotTo(gomega.BeNil())
		})

		ginkgo.It("Should not setup secret when source is local cluster", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "https://destination", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should NOT be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).To(gomega.BeNil())
		})
	})

	ginkgo.Describe("GuestToolsIssue aggregation", func() {
		var (
			mockValidator   *mockGuestToolsValidator
			guestToolsIssue libcnd.Condition
		)

		ginkgo.BeforeEach(func() {
			mockValidator = &mockGuestToolsValidator{
				responses: make(map[string]guestToolsResponse),
			}
			guestToolsIssue = libcnd.Condition{
				Type:     GuestToolsIssue,
				Status:   libcnd.True,
				Reason:   NotValid,
				Category: api.CategoryCritical,
				Message:  "",
				Items:    []string{},
			}
		})

		ginkgo.It("should append multiple failing VMs to Items", func() {
			// Setup multiple VMs with guest tools issues
			mockValidator.responses["vm1"] = guestToolsResponse{ok: false, msg: "VM1 tools not installed"}
			mockValidator.responses["vm2"] = guestToolsResponse{ok: false, msg: "VM2 tools not running"}
			mockValidator.responses["vm3"] = guestToolsResponse{ok: true, msg: ""}

			refs := []ref.Ref{
				{Name: "vm1", Namespace: "test"},
				{Name: "vm2", Namespace: "test"},
				{Name: "vm3", Namespace: "test"},
			}

			// Simulate the validation loop
			for _, vmRef := range refs {
				ok, err := mockValidator.GuestToolsInstalled(vmRef)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				if !ok {
					guestToolsIssue.Items = append(guestToolsIssue.Items, vmRef.String())
				}
			}

			// Verify that both failing VMs are in Items
			gomega.Expect(guestToolsIssue.Items).To(gomega.HaveLen(2))
			gomega.Expect(guestToolsIssue.Items).To(gomega.ContainElement(" id: name:'vm1' "))
			gomega.Expect(guestToolsIssue.Items).To(gomega.ContainElement(" id: name:'vm2' "))
			gomega.Expect(guestToolsIssue.Items).NotTo(gomega.ContainElement(" id: name:'vm3' "))

			// Generic message is now used from condition level
			gomega.Expect(guestToolsIssue.Message).To(gomega.Equal(""))
		})

		ginkgo.It("should add failing VM to Items with generic guidance", func() {
			// Setup VM with specific tools issue
			mockValidator.responses["encrypted-vm"] = guestToolsResponse{
				ok:  false,
				msg: "Unable to determine VMware Tools status for this powered-on VM. This commonly occurs when an encrypted VM is locked and VMware Tools cannot start. Power off the VM manually (or unlock the disks) before migration.",
			}

			vmRef := ref.Ref{Name: "encrypted-vm", Namespace: "test"}
			ok, err := mockValidator.GuestToolsInstalled(vmRef)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ok).To(gomega.BeFalse())

			guestToolsIssue.Items = append(guestToolsIssue.Items, vmRef.String())

			// Verify VM is added to Items (message is now generic at condition level)
			gomega.Expect(guestToolsIssue.Items).To(gomega.ContainElement(" id: name:'encrypted-vm' "))
		})

		ginkgo.It("should add failing VMs to Items regardless of provider type", func() {
			// Setup VM that returns empty message (e.g., from non-VSphere providers)
			mockValidator.responses["vm-empty-msg"] = guestToolsResponse{ok: false, msg: ""}

			vmRef := ref.Ref{Name: "vm-empty-msg", Namespace: "test"}
			ok, err := mockValidator.GuestToolsInstalled(vmRef)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ok).To(gomega.BeFalse())

			guestToolsIssue.Items = append(guestToolsIssue.Items, vmRef.String())

			// Verify VM is added to Items (providers no longer return messages)
			gomega.Expect(guestToolsIssue.Items).To(gomega.ContainElement(" id: name:'vm-empty-msg' "))
		})

		ginkgo.It("should add all failing VMs to Items with generic message", func() {
			// Setup multiple VMs where first one has detailed message
			mockValidator.responses["first-vm"] = guestToolsResponse{
				ok:  false,
				msg: "First VM detailed error message",
			}
			mockValidator.responses["second-vm"] = guestToolsResponse{
				ok:  false,
				msg: "Second VM error message",
			}

			refs := []ref.Ref{
				{Name: "first-vm", Namespace: "test"},
				{Name: "second-vm", Namespace: "test"},
			}

			// Simulate the validation loop
			for _, vmRef := range refs {
				ok, err := mockValidator.GuestToolsInstalled(vmRef)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				if !ok {
					guestToolsIssue.Items = append(guestToolsIssue.Items, vmRef.String())
				}
			}

			// Verify both VMs are added to Items (messages are now generic at condition level)
			gomega.Expect(guestToolsIssue.Items).To(gomega.HaveLen(2))
		})
	})

	ginkgo.Describe("validateTransferNetwork", func() {
		ginkgo.It("should pass validation when route annotation has valid IP", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: "192.168.1.1",
					},
				},
			}

			reconciler := createFakeReconciler(nad)
			plan := &api.Plan{
				Spec: api.PlanSpec{
					TransferNetwork: &core.ObjectReference{
						Namespace: "test-ns",
						Name:      "test-nad",
					},
				},
			}

			err := reconciler.validateTransferNetwork(plan)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(TransferNetNotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should pass validation when route annotation is 'none'", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: AnnForkliftRouteValueNone,
					},
				},
			}

			reconciler := createFakeReconciler(nad)
			plan := &api.Plan{
				Spec: api.PlanSpec{
					TransferNetwork: &core.ObjectReference{
						Namespace: "test-ns",
						Name:      "test-nad",
					},
				},
			}

			err := reconciler.validateTransferNetwork(plan)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(TransferNetNotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should set warning when route annotation is missing", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
				},
			}

			reconciler := createFakeReconciler(nad)
			plan := &api.Plan{
				Spec: api.PlanSpec{
					TransferNetwork: &core.ObjectReference{
						Namespace: "test-ns",
						Name:      "test-nad",
					},
				},
			}

			err := reconciler.validateTransferNetwork(plan)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(TransferNetMissingDefaultRoute)).To(gomega.BeTrue())
		})

		ginkgo.It("should set error when route annotation has invalid IP", func() {
			nad := &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-nad",
					Namespace: "test-ns",
					Annotations: map[string]string{
						AnnForkliftNetworkRoute: "invalid-ip-address",
					},
				},
			}

			reconciler := createFakeReconciler(nad)
			plan := &api.Plan{
				Spec: api.PlanSpec{
					TransferNetwork: &core.ObjectReference{
						Namespace: "test-ns",
						Name:      "test-nad",
					},
				},
			}

			err := reconciler.validateTransferNetwork(plan)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(TransferNetNotValid)).To(gomega.BeTrue())
			gomega.Expect(plan.Status.FindCondition(TransferNetNotValid).Reason).To(gomega.Equal(NotValid))
		})

		ginkgo.It("should set error when NAD does not exist", func() {
			reconciler := createFakeReconciler()
			plan := &api.Plan{
				Spec: api.PlanSpec{
					TransferNetwork: &core.ObjectReference{
						Namespace: "test-ns",
						Name:      "non-existent-nad",
					},
				},
			}

			err := reconciler.validateTransferNetwork(plan)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(TransferNetNotValid)).To(gomega.BeTrue())
			gomega.Expect(plan.Status.FindCondition(TransferNetNotValid).Reason).To(gomega.Equal(NotFound))
		})
	})
})

var _ = ginkgo.Describe("checkDiskMigrationTypesDetailed", func() {
	var (
		reconciler *Reconciler
	)

	ginkgo.BeforeEach(func() {
		reconciler = createFakeReconciler()
	})

	// Helper to create a vsphere.VM with disks
	createVSphereVM := func(name string, diskDatastores []string) *vsphere.VM {
		disks := []vspheremodel.Disk{}
		for i, dsID := range diskDatastores {
			disks = append(disks, vspheremodel.Disk{
				Key: int32(i + 2000),
				Datastore: vspheremodel.Ref{
					ID: dsID,
				},
			})
		}
		return &vsphere.VM{
			VM1: vsphere.VM1{
				VM0: vsphere.VM0{
					ID:   name + "-id",
					Path: name,
				},
				Disks: disks,
			},
		}
	}

	// Helper to create a StorageMap
	createStorageMap := func(datastorePairs []struct {
		datastoreID string
		hasOffload  bool
	}) *api.StorageMap {
		pairs := []api.StoragePair{}
		for _, pair := range datastorePairs {
			sp := api.StoragePair{
				Source: ref.Ref{
					ID: pair.datastoreID,
				},
				Destination: api.DestinationStorage{
					StorageClass: "test-storage-class",
				},
			}
			if pair.hasOffload {
				sp.OffloadPlugin = &api.OffloadPlugin{
					VSphereXcopyPluginConfig: &api.VSphereXcopyPluginConfig{
						StorageVendorProduct: api.StorageVendorProduct("test-vendor"),
					},
				}
			}
			pairs = append(pairs, sp)
		}
		return &api.StorageMap{
			Spec: api.StorageMapSpec{
				Map: pairs,
			},
		}
	}

	// SHOULD PASS cases - these return hasOffload=false,hasVddk=true OR hasOffload=true,hasVddk=false (not both)
	ginkgo.DescribeTable("should identify pure disk types correctly",
		func(vmName string, diskDatastores []string, storageMapPairs []struct {
			datastoreID string
			hasOffload  bool
		}, expectedHasOffload, expectedHasVddk bool) {
			storageMap := createStorageMap(storageMapPairs)
			vsphereVM := createVSphereVM(vmName, diskDatastores)

			diskDetails, hasOffload, hasVddk, err := reconciler.checkDiskMigrationTypesDetailed(storageMap, vsphereVM, vmName)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(hasOffload).To(gomega.Equal(expectedHasOffload))
			gomega.Expect(hasVddk).To(gomega.Equal(expectedHasVddk))
			gomega.Expect(len(diskDetails)).To(gomega.Equal(len(diskDatastores)))
		},
		ginkgo.Entry("one pure VDDK disk",
			"vm1",
			[]string{"ds1"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: false},
			},
			false, // hasOffload
			true,  // hasVddk
		),
		ginkgo.Entry("one pure offload disk",
			"vm1",
			[]string{"ds1"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: true},
			},
			true,  // hasOffload
			false, // hasVddk
		),
		ginkgo.Entry("multiple pure VDDK disks",
			"vm1",
			[]string{"ds1", "ds2"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: false},
				{datastoreID: "ds2", hasOffload: false},
			},
			false, // hasOffload
			true,  // hasVddk
		),
		ginkgo.Entry("multiple pure offload disks",
			"vm1",
			[]string{"ds1", "ds2"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: true},
				{datastoreID: "ds2", hasOffload: true},
			},
			true,  // hasOffload
			false, // hasVddk
		),
	)

	// SHOULD FAIL cases - these return both hasOffload=true AND hasVddk=true
	ginkgo.DescribeTable("should identify mixed disk types correctly",
		func(vmName string, diskDatastores []string, storageMapPairs []struct {
			datastoreID string
			hasOffload  bool
		}) {
			storageMap := createStorageMap(storageMapPairs)
			vsphereVM := createVSphereVM(vmName, diskDatastores)

			diskDetails, hasOffload, hasVddk, err := reconciler.checkDiskMigrationTypesDetailed(storageMap, vsphereVM, vmName)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(hasOffload).To(gomega.BeTrue(), "should detect offload disks")
			gomega.Expect(hasVddk).To(gomega.BeTrue(), "should detect VDDK disks")
			gomega.Expect(len(diskDetails)).To(gomega.Equal(len(diskDatastores)))

			// Verify disk details contain both types
			hasOffloadInDetails := false
			hasVddkInDetails := false
			for _, disk := range diskDetails {
				if disk.Type == DiskTypeOffload {
					hasOffloadInDetails = true
				} else if disk.Type == DiskTypeVDDK {
					hasVddkInDetails = true
				}
			}
			gomega.Expect(hasOffloadInDetails).To(gomega.BeTrue())
			gomega.Expect(hasVddkInDetails).To(gomega.BeTrue())
		},
		ginkgo.Entry("mixed VM with both VDDK and offload disks",
			"vm1",
			[]string{"ds1", "ds2"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: false}, // VDDK
				{datastoreID: "ds2", hasOffload: true},  // Offload
			},
		),
		ginkgo.Entry("mixed VM with multiple disks of each type",
			"vm1",
			[]string{"ds1", "ds2", "ds3", "ds4"},
			[]struct {
				datastoreID string
				hasOffload  bool
			}{
				{datastoreID: "ds1", hasOffload: false}, // VDDK
				{datastoreID: "ds2", hasOffload: true},  // Offload
				{datastoreID: "ds3", hasOffload: false}, // VDDK
				{datastoreID: "ds4", hasOffload: true},  // Offload
			},
		),
	)
})

var _ = ginkgo.Describe("validateVddkAndOffloadMixedUsage condition logic", func() {
	// This is a placeholder for future tests
	// Note: This test would need mocking of web.NewClient
	// For now, we test the core logic via checkDiskMigrationTypesDetailed above
	// Full integration test would require dependency injection or build tags
	ginkgo.It("should set condition when mixed VMs detected", ginkgo.Pending, func() {
		// TODO: Requires mocking web.NewClient
	})
})

// Mock validator for testing GuestToolsIssue aggregation
type guestToolsResponse struct {
	ok  bool
	msg string
	err error
}

type mockGuestToolsValidator struct {
	responses map[string]guestToolsResponse
}

func (m *mockGuestToolsValidator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	if response, exists := m.responses[vmRef.Name]; exists {
		return response.ok, response.err
	}
	// Default: tools are OK
	return true, nil
}

//nolint:errcheck
func createFakeReconciler(objects ...runtime.Object) *Reconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = k8snet.AddToScheme(scheme)
	api.SchemeBuilder.AddToScheme(scheme)

	client := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	return &Reconciler{
		base.Reconciler{
			Client: client,
			Log:    planValidationLog,
		},
		client,
	}
}

func createProvider(name, namespace, url string, providerType api.ProviderType, secret *core.ObjectReference) *api.Provider {
	return &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.ProviderSpec{
			Type:   ptr.To(providerType),
			URL:    url,
			Secret: *secret,
		},
	}
}

func createSecret(name, namespace string, insecure bool) *core.Secret {
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			insecureSkipKey: []byte(strconv.FormatBool(insecure)),
			tokenKey:        []byte(tokenValue),
		},
	}
}

func createPlan(name, namespace string, source, destination *api.Provider) *api.Plan {
	return &api.Plan{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.PlanSpec{
			Provider: provider.Pair{
				Source: core.ObjectReference{
					Name:      source.Name,
					Namespace: source.Namespace,
				},
				Destination: core.ObjectReference{
					Name:      destination.Name,
					Namespace: destination.Namespace,
				},
			},
		},
		Referenced: api.Referenced{
			Provider: struct {
				Source      *api.Provider
				Destination *api.Provider
			}{
				Source:      source,
				Destination: destination,
			},
		},
	}
}
