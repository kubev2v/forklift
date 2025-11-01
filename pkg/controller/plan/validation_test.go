package plan

import (
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
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

func (m *mockGuestToolsValidator) GuestToolsInstalled(vmRef ref.Ref) (bool, error) {
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
