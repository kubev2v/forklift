package plan

import (
	"strconv"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/provider"
	"github.com/konveyor/forklift-controller/pkg/controller/base"
	"github.com/konveyor/forklift-controller/pkg/lib/condition"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	discovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			secret := createSecret("source-secret", "source-namespace", false)
			source := createProvider("source", "source-namespace", "https://source", v1beta1.OpenShift, &core.ObjectReference{Name: "source-secret", Namespace: "source-namespace"})
			destination := createProvider("destination", "destination-namespace", "", v1beta1.OpenShift, &core.ObjectReference{})
			plan := createPlan("test-plan", "test-namespace", source, destination)
			source.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})
			destination.Status.Conditions.SetCondition(condition.Condition{Type: condition.Ready, Status: condition.True})

			reconciler = createFakeReconciler(secret, plan, source, destination)
			err := reconciler.ensureSecretForProvider(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// secret should be set on plan.Referenced.Secret
			gomega.Expect(plan.Referenced.Secret).NotTo(gomega.BeNil())
		})

		ginkgo.It("Should not setup secret when source is local cluster", func() {
			secret := createSecret("source-secret", "source-namespace", false)
			source := createProvider("source", "source-namespace", "", v1beta1.OpenShift, &core.ObjectReference{Name: "source-secret", Namespace: "source-namespace"})
			destination := createProvider("destination", "destination-namespace", "https://destination", v1beta1.OpenShift, &core.ObjectReference{})
			plan := createPlan("test-plan", "test-namespace", source, destination)
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

//nolint:errcheck
func createFakeReconciler(objects ...runtime.Object) *Reconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)

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

func createProvider(name, namespace, url string, providerType v1beta1.ProviderType, secret *core.ObjectReference) *v1beta1.Provider {
	return &v1beta1.Provider{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.ProviderSpec{
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
			"inscureSkipVerify": []byte(strconv.FormatBool(insecure)),
			"token":             []byte("token"),
		},
	}
}

func createPlan(name, namespace string, source, destination *api.Provider) *v1beta1.Plan {
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
		Referenced: v1beta1.Referenced{
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
