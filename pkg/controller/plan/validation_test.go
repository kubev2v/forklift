package plan

import (
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
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
			source.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})
			destination.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})

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
			source.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})
			destination.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should NOT be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).To(gomega.BeNil())
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
