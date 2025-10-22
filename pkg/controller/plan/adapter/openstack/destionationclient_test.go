//nolint:errcheck
package openstack

import (
	"context"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var destinationClientLog = logging.WithName("openstack-destinationclient-test")

var _ = Describe("openstack destinationclient tests", func() {
	destinationClient := createDestinationClient()
	openstackVolPopCr := &v1beta1.OpenstackVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels: map[string]string{
				"migration": "migration1",
				"imageID":   "image1",
			},
		},
		Spec: v1beta1.OpenstackVolumePopulatorSpec{
			ImageID: "image1",
		},
	}

	pvc1 := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testPVC",
			Namespace: "test",
			Labels: map[string]string{
				"migration": "migration1",
				"imageID":   "image1",
			},
		},
	}

	Describe("findPvcByCr", func() {
		It("should return an error when PVC is not found", func() {
			pvc, err := destinationClient.findPVCByCR(openstackVolPopCr)
			Expect(pvc).To(BeNil())
			Expect(err).To(MatchError("PVC not found"))
		})

		It("should return the PVC when it is found", func() {
			destinationClient = createDestinationClient(pvc1)
			pvc, err := destinationClient.findPVCByCR(openstackVolPopCr)
			Expect(pvc).NotTo(BeNil())
			Expect(pvc.Name).To(Equal("testPVC"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error when multiple PVCs are found", func() {
			pvc2 := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testPVC2",
					Namespace: "test",
					Labels: map[string]string{
						"migration": "migration1",
						"imageID":   "image1",
					},
				},
			}
			destinationClient = createDestinationClient(pvc1, pvc2)
			pvc, err := destinationClient.findPVCByCR(openstackVolPopCr)
			Expect(pvc).To(BeNil())
			Expect(err).To(MatchError("Multiple PVCs found"))
		})
	})

	Describe("SetPopulatorCrOwnership", func() {
		It("should set the owner reference for the populator CR", func() {
			destinationClient = createDestinationClient(openstackVolPopCr, pvc1)
			destinationClient.SetPopulatorCrOwnership()

			patchedOpenstackVolPopCr := &v1beta1.OpenstackVolumePopulator{}
			err := destinationClient.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "test",
				Namespace: "test",
			}, patchedOpenstackVolPopCr)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchedOpenstackVolPopCr.GetOwnerReferences()).To(HaveLen(1))
			Expect(patchedOpenstackVolPopCr.GetOwnerReferences()[0].Kind).To(Equal("PersistentVolumeClaim"))
			Expect(patchedOpenstackVolPopCr.GetOwnerReferences()[0].Name).To(Equal("testPVC"))
		})
	})
})

func createDestinationClient(objs ...runtime.Object) *DestinationClient {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &DestinationClient{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Plan: createPlan(),
			Log:  destinationClientLog,

			// To make sure r.Scheme is not nil
			Client: client,
		},
	}
}

func createPlan() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1beta1.PlanSpec{
			TargetNamespace: "test",
		},
		Status: v1beta1.PlanStatus{
			Migration: plan.MigrationStatus{
				History: []plan.Snapshot{
					{
						Migration: plan.SnapshotRef{
							UID: "migration1",
						},
					},
				},
			},
		},
	}
}
