//nolint:errcheck
package ovirt

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

var destinationClientLog = logging.WithName("ovirt-destinationclient-test")

var _ = Describe("ovirt destinationclient tests", func() {
	destinationClient := createDestinationClient()
	ovirtVolPopCr := &v1beta1.OvirtVolumePopulator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels: map[string]string{
				"migration": "migration1",
				"diskID":    "disk1",
			},
		},
		Spec: v1beta1.OvirtVolumePopulatorSpec{
			DiskID: "disk1",
		},
	}

	pvc1 := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testPVC",
			Namespace: "test",
			Labels: map[string]string{
				"migration": "migration1",
				"diskID":    "disk1",
			},
		},
	}

	Describe("findPvcByCr", func() {
		It("should return an error when PVC is not found", func() {
			pvc, err := destinationClient.findPVCByCR(ovirtVolPopCr)
			Expect(pvc).To(BeNil())
			Expect(err).To(MatchError("PVC not found"))
		})

		It("should return the PVC when it is found", func() {
			destinationClient = createDestinationClient(pvc1)
			pvc, err := destinationClient.findPVCByCR(ovirtVolPopCr)
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
						"diskID":    "disk1",
					},
				},
			}
			destinationClient = createDestinationClient(pvc1, pvc2)
			pvc, err := destinationClient.findPVCByCR(ovirtVolPopCr)
			Expect(pvc).To(BeNil())
			Expect(err).To(MatchError("Multiple PVCs found"))
		})
	})

	Describe("SetPopulatorCrOwnership", func() {
		It("should set the owner reference for the populator CR", func() {
			destinationClient = createDestinationClient(ovirtVolPopCr, pvc1)
			destinationClient.SetPopulatorCrOwnership()

			patchedOvirtVolPopCr := &v1beta1.OvirtVolumePopulator{}
			err := destinationClient.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "test",
				Namespace: "test",
			}, patchedOvirtVolPopCr)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchedOvirtVolPopCr.GetOwnerReferences()).To(HaveLen(1))
			Expect(patchedOvirtVolPopCr.GetOwnerReferences()[0].Kind).To(Equal("PersistentVolumeClaim"))
			Expect(patchedOvirtVolPopCr.GetOwnerReferences()[0].Name).To(Equal("testPVC"))
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
