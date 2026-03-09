//nolint:errcheck
package plan

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var KubeVirtLog = logging.WithName("kubevirt-test")

var _ = ginkgo.Describe("kubevirt tests", func() {
	ginkgo.Describe("getPVCs", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "test",
					"vmID":      "test",
				},
			},
		}

		ginkgo.It("should return PVCs", func() {
			kubevirt := createKubeVirt(pvc)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
		})
	})

	ginkgo.Describe("getVirtV2vImage", func() {
		const globalImage = "quay.io/kubev2v/forklift-virt-v2v:latest"
		const xfsImage = "quay.io/kubev2v/forklift-virt-v2v-rhel9:latest"

		ginkgo.BeforeEach(func() {
			Settings.Migration.VirtV2vImage = globalImage
			Settings.Migration.VirtV2vImageXFS = xfsImage
		})

		ginkgo.It("should return the global image when plan has no override", func() {
			p := createPlanKubevirt()
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should return the per-plan image when set", func() {
			perPlanImage := "quay.io/kubev2v/forklift-virt-v2v:custom-build"
			p := createPlanKubevirt()
			p.Spec.VirtV2vImage = perPlanImage
			Expect(getVirtV2vImage(p)).To(Equal(perPlanImage))
		})

		ginkgo.It("should fall back to global image when plan override is empty string", func() {
			p := createPlanKubevirt()
			p.Spec.VirtV2vImage = ""
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should return the XFS image when XfsCompatibility is enabled", func() {
			p := createPlanKubevirt()
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(xfsImage))
		})

		ginkgo.It("should return the global image when XfsCompatibility is false", func() {
			p := createPlanKubevirt()
			p.Spec.XfsCompatibility = false
			Expect(getVirtV2vImage(p)).To(Equal(globalImage))
		})

		ginkgo.It("should prioritize VirtV2vImage over XfsCompatibility when both are set", func() {
			perPlanImage := "quay.io/kubev2v/forklift-virt-v2v:custom-build"
			p := createPlanKubevirt()
			p.Spec.VirtV2vImage = perPlanImage
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(perPlanImage))
		})

		ginkgo.It("should return XFS image when XfsCompatibility is true and VirtV2vImage is empty", func() {
			p := createPlanKubevirt()
			p.Spec.VirtV2vImage = ""
			p.Spec.XfsCompatibility = true
			Expect(getVirtV2vImage(p)).To(Equal(xfsImage))
		})
	})
})

func createKubeVirt(objs ...runtime.Object) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(),
			Client:    client,
		},
	}
}

func createMigration() *v1beta1.Migration {
	return &v1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "test",
		},
	}
}
func createPlanKubevirt() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1beta1.PlanSpec{
			Type: "cold",
		},
	}
}
