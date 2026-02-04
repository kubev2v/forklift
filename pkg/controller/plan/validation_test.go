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

	ginkgo.Describe("validateConversionTempStorage", func() {
		ginkgo.It("should pass when both fields are set", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			plan.Spec.ConversionTempStorageClass = "fast-ssd"
			plan.Spec.ConversionTempStorageSize = "50Gi"
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.validateConversionTempStorage(plan)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should pass when neither field is set", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.validateConversionTempStorage(plan)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should fail when only storage class is set", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			plan.Spec.ConversionTempStorageClass = "fast-ssd"
			plan.Spec.ConversionTempStorageSize = ""
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.validateConversionTempStorage(plan)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeTrue())
			condition := plan.Status.FindCondition(NotValid)
			gomega.Expect(condition).NotTo(gomega.BeNil())
			gomega.Expect(condition.Message).To(gomega.ContainSubstring("Both ConversionTempStorageClass and ConversionTempStorageSize must be specified together"))
		})

		ginkgo.It("should fail when only storage size is set", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			plan.Spec.ConversionTempStorageClass = ""
			plan.Spec.ConversionTempStorageSize = "50Gi"
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.validateConversionTempStorage(plan)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeTrue())
			condition := plan.Status.FindCondition(NotValid)
			gomega.Expect(condition).NotTo(gomega.BeNil())
			gomega.Expect(condition.Message).To(gomega.ContainSubstring("Both ConversionTempStorageClass and ConversionTempStorageSize must be specified together"))
		})

		ginkgo.It("should fail when storage size is invalid", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			plan := createPlan(testPlanName, testNamespace, source, destination)
			plan.Spec.ConversionTempStorageClass = "fast-ssd"
			plan.Spec.ConversionTempStorageSize = "invalid-size"
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.validateConversionTempStorage(plan)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeTrue())
			condition := plan.Status.FindCondition(NotValid)
			gomega.Expect(condition).NotTo(gomega.BeNil())
			gomega.Expect(condition.Message).To(gomega.ContainSubstring("is not a valid Kubernetes resource quantity"))
		})

		ginkgo.It("should pass with valid size formats", func() {
			secret := createSecret(sourceSecretName, sourceNamespace, false)
			source := createProvider(sourceName, sourceNamespace, "https://source", api.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", api.OpenShift, &core.ObjectReference{})
			source.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})
			destination.Status.Conditions.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True})

			validSizes := []string{"50Gi", "1Ti", "100Mi", "500G", "2T"}
			for _, size := range validSizes {
				plan := createPlan(testPlanName, testNamespace, source, destination)
				plan.Spec.ConversionTempStorageClass = "fast-ssd"
				plan.Spec.ConversionTempStorageSize = size

				reconciler = createFakeReconciler(secret, plan, source, destination)
				err := reconciler.validateConversionTempStorage(plan)

				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Size %s should be valid", size)
				gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse(), "Size %s should not cause validation error", size)
			}
		})
	})
})

var _ = ginkgo.Describe("vmUsesVddk", func() {
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

	// Tests for VDDK usage detection
	ginkgo.DescribeTable("should correctly identify if VM uses VDDK",
		func(vmName string, diskDatastores []string, storageMapPairs []struct {
			datastoreID string
			hasOffload  bool
		}, expectedUsesVddk bool) {
			storageMap := createStorageMap(storageMapPairs)
			vsphereVM := createVSphereVM(vmName, diskDatastores)

			usesVddk, err := reconciler.vmUsesVddk(storageMap, vsphereVM, vmName)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(usesVddk).To(gomega.Equal(expectedUsesVddk))
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
			true, // uses VDDK
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
			false, // doesn't use VDDK (uses offload)
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
			true, // uses VDDK
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
			false, // doesn't use VDDK (uses offload)
		),
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
			true, // uses VDDK (because at least one disk uses VDDK)
		),
	)
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

var _ = ginkgo.Describe("Template Validation", func() {
	var reconciler *Reconciler

	ginkgo.BeforeEach(func() {
		reconciler = createFakeReconciler()
	})

	ginkgo.Describe("IsValidVolumeNameTemplate", func() {
		ginkgo.It("should pass with empty template", func() {
			err := reconciler.IsValidVolumeNameTemplate("")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with valid simple template", func() {
			err := reconciler.IsValidVolumeNameTemplate("disk-{{.VolumeIndex}}")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with template using PVCName", func() {
			err := reconciler.IsValidVolumeNameTemplate("vol-{{.PVCName}}")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with invalid template syntax", func() {
			err := reconciler.IsValidVolumeNameTemplate("disk-{{.InvalidField}}")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with unclosed template braces", func() {
			err := reconciler.IsValidVolumeNameTemplate("disk-{{.VolumeIndex")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail when output is not valid DNS label", func() {
			// Template that produces output starting with hyphen
			err := reconciler.IsValidVolumeNameTemplate("-{{.PVCName}}")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("IsValidNetworkNameTemplate", func() {
		ginkgo.It("should pass with empty template", func() {
			err := reconciler.IsValidNetworkNameTemplate("")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with valid simple template", func() {
			err := reconciler.IsValidNetworkNameTemplate("net-{{.NetworkIndex}}")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with template using NetworkName", func() {
			err := reconciler.IsValidNetworkNameTemplate("{{.NetworkName}}")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with invalid template syntax", func() {
			err := reconciler.IsValidNetworkNameTemplate("net-{{.InvalidField}}")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with unclosed template braces", func() {
			err := reconciler.IsValidNetworkNameTemplate("net-{{.NetworkIndex")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("IsValidTargetName", func() {
		ginkgo.It("should pass with empty target name", func() {
			err := reconciler.IsValidTargetName("")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with valid DNS subdomain name", func() {
			err := reconciler.IsValidTargetName("my-vm-name")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass with name containing dots", func() {
			err := reconciler.IsValidTargetName("my.vm.name")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with name starting with hyphen", func() {
			err := reconciler.IsValidTargetName("-invalid-name")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with name containing uppercase", func() {
			err := reconciler.IsValidTargetName("Invalid-Name")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with name containing spaces", func() {
			err := reconciler.IsValidTargetName("invalid name")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with name containing underscores", func() {
			err := reconciler.IsValidTargetName("invalid_name")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("validateVolumeNameTemplate", func() {
		ginkgo.It("should not set condition for empty template", func() {
			plan := &api.Plan{
				Spec: api.PlanSpec{
					VolumeNameTemplate: "",
				},
			}
			err := reconciler.validateVolumeNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should not set condition for valid template", func() {
			plan := &api.Plan{
				Spec: api.PlanSpec{
					VolumeNameTemplate: "disk-{{.VolumeIndex}}",
				},
			}
			err := reconciler.validateVolumeNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should set condition for invalid template", func() {
			plan := &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-plan",
					Namespace: "test-ns",
				},
				Spec: api.PlanSpec{
					VolumeNameTemplate: "{{.InvalidField}}",
				},
			}
			err := reconciler.validateVolumeNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("validateNetworkNameTemplate", func() {
		ginkgo.It("should not set condition for empty template", func() {
			plan := &api.Plan{
				Spec: api.PlanSpec{
					NetworkNameTemplate: "",
				},
			}
			err := reconciler.validateNetworkNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should not set condition for valid template", func() {
			plan := &api.Plan{
				Spec: api.PlanSpec{
					NetworkNameTemplate: "net-{{.NetworkIndex}}",
				},
			}
			err := reconciler.validateNetworkNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeFalse())
		})

		ginkgo.It("should set condition for invalid template", func() {
			plan := &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-plan",
					Namespace: "test-ns",
				},
				Spec: api.PlanSpec{
					NetworkNameTemplate: "{{.InvalidField}}",
				},
			}
			err := reconciler.validateNetworkNameTemplate(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.HasCondition(NotValid)).To(gomega.BeTrue())
		})
	})
})
