package plan

import (
	"strconv"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
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
			source := createProvider(sourceName, sourceNamespace, "https://source", v1beta1.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "", v1beta1.OpenShift, &core.ObjectReference{})
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
			source := createProvider(sourceName, sourceNamespace, "", v1beta1.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
			destination := createProvider(destName, destNamespace, "https://destination", v1beta1.OpenShift, &core.ObjectReference{})
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

	ginkgo.Describe("validatePVCNameTemplate", func() {
		var reconciler *Reconciler

		ginkgo.BeforeEach(func() {
			reconciler = &Reconciler{
				Reconciler: base.Reconciler{
					Log: planValidationLog,
				},
			}
		})

		source := createProvider(sourceName, sourceNamespace, "", v1beta1.OpenShift, &core.ObjectReference{Name: sourceSecretName, Namespace: sourceNamespace})
		destination := createProvider(destName, destNamespace, "https://destination", v1beta1.OpenShift, &core.ObjectReference{})

		ginkgo.DescribeTable("should validate a plan correctly",
			func(template string, shouldBeValid bool) {
				plan := createPlan(testPlanName, testNamespace, source, destination)
				plan.Spec.PVCNameTemplate = template

				err := reconciler.validatePVCNameTemplate(plan)
				if err != nil {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				hasInvalidCondition := false
				for _, cond := range plan.Status.Conditions.List {
					if cond.Type == NotValid {
						hasInvalidCondition = true
						break
					}
				}

				if shouldBeValid {
					gomega.Expect(hasInvalidCondition).To(gomega.BeFalse())
				} else {
					gomega.Expect(hasInvalidCondition).To(gomega.BeTrue())
				}
			},
			ginkgo.Entry("empty template is valid", "", true),
			ginkgo.Entry("simple valid template", "{{.VmName}}-disk-{{.DiskIndex}}", true),
			ginkgo.Entry("complex valid template", "{{.PlanName}}-{{.VmName}}-disk-{{.DiskIndex}}", true),
			ginkgo.Entry("valid template with root disk index", "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}", true),
			ginkgo.Entry("template with invalid k8s label chars", "disk@{{.DiskIndex}}", false),
			ginkgo.Entry("template with undefined variable", "{{.UndefinedVar}}", false),
			ginkgo.Entry("template resulting in empty string", "{{if false}}disk{{end}}", false),
			ginkgo.Entry("template with special characters", "disk!{{.DiskIndex}}", false),
			ginkgo.Entry("template with spaces", "disk {{.DiskIndex}}", false),
			ginkgo.Entry("template with invalid start character", "_{{.VmName}}", false),
			ginkgo.Entry("template exceeding length limit", "very-very-very-very-very-very-very-very-very-very-long-prefix-{{.VmName}}", false),
			ginkgo.Entry("template with slash character", "{{.VmName}}/{{.DiskIndex}}", false),
		)
	})

	ginkgo.Describe("IsValidPVCNameTemplate", func() {
		var reconciler *Reconciler

		ginkgo.BeforeEach(func() {
			reconciler = &Reconciler{
				Reconciler: base.Reconciler{
					Log: planValidationLog,
				},
			}
		})

		ginkgo.DescribeTable("should validate PVC name template correctly",
			func(template string, shouldBeValid bool) {
				err := reconciler.IsValidPVCNameTemplate(template)
				if shouldBeValid {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				} else {
					gomega.Expect(err).To(gomega.HaveOccurred())
				}
			},
			ginkgo.Entry("empty template is valid", "", true),
			ginkgo.Entry("simple valid template", "{{.VmName}}-disk-{{.DiskIndex}}", true),
			ginkgo.Entry("complex valid template", "{{.PlanName}}-{{.VmName}}-disk-{{.DiskIndex}}", true),
			ginkgo.Entry("valid template with root disk index", "{{if eq .DiskIndex .RootDiskIndex}}root{{else}}data{{end}}-{{.DiskIndex}}", true),
			ginkgo.Entry("invalid template syntax", "{{.VmName}-disk-{{.DiskIndex}", false),
			ginkgo.Entry("template with invalid k8s label chars", "disk@{{.DiskIndex}}", false),
			ginkgo.Entry("template with undefined variable", "{{.UndefinedVar}}", false),
			ginkgo.Entry("template resulting in empty string", "{{if false}}disk{{end}}", false),
			ginkgo.Entry("template starting with non-alphanumeric", "-{{.VmName}}", false),
			ginkgo.Entry("template ending with non-alphanumeric", "{{.VmName}}-", false),
			ginkgo.Entry("template with too long result", "very-long-prefix-that-will-definitely-exceed-kubernetes-label-length-limit-for-sure-{{.VmName}}", false),
			ginkgo.Entry("template with invalid character in the middle", "disk-{{.VmName}}/{{.DiskIndex}}", false),
			ginkgo.Entry("template with uppercase characters (invalid K8s name)", "DISK-{{.VmName}}", false),
		)
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
			insecureSkipKey: []byte(strconv.FormatBool(insecure)),
			tokenKey:        []byte(tokenValue),
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
