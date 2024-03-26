package adapter

import (
	"context"

	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	converterLog = logging.WithName("converter-test")
)

var _ = Describe("Converter tests", func() {
	var (
		converter *Converter
	)

	const (
		pvcName      = "test-pvc"
		pvcNamespace = "test-namespace"
	)

	var _ = Describe("Job status", func() {
		qcow2PVC := &v1.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      pvcName,
				Namespace: pvcNamespace,
				Annotations: map[string]string{
					base.AnnSourceFormat: "qcow2",
				},
			},
		}

		srcFormatFn := func(pvc *v1.PersistentVolumeClaim) string {
			return pvc.Annotations[base.AnnSourceFormat]
		}

		It("Should not be ready if job is not ready", func() {
			convertJob := createFakeConvertJob(qcow2PVC, pvcNamespace)
			converter = createFakeConverter(qcow2PVC, convertJob)
			ready, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(ready).To(BeFalse())
		})

		It("Should be ready if job is ready", func() {
			convertJob := createFakeConvertJob(qcow2PVC, pvcNamespace)
			convertJob.Status.Succeeded = 1

			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-dv",
					Namespace: pvcNamespace,
					Labels: map[string]string{
						base.AnnConversionSourcePVC: pvcName,
					},
				},
			}

			dv.Status.Phase = cdi.Succeeded

			converter = createFakeConverter(qcow2PVC, convertJob, dv)
			ready, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(ready).To(BeTrue())
		})

		It("Should create job if it does not exist", func() {
			converter = createFakeConverter(qcow2PVC)
			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-dv",
					Namespace: pvcNamespace,
				},
			}
			job, err := converter.ensureJob(qcow2PVC, dv, srcFormatFn(qcow2PVC), "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(job).ToNot(BeNil())
		})

		It("Should create scratch DV if it does not exist", func() {
			converter = createFakeConverter(qcow2PVC)
			dv, err := converter.ensureScratchDV(qcow2PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(dv).ToNot(BeNil())
		})

		It("Should remove scratch DV if the job failed", func() {
			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-dv",
					Namespace: pvcNamespace,
					Labels: map[string]string{
						base.AnnConversionSourcePVC: pvcName,
					},
				},
				Status: cdi.DataVolumeStatus{
					Phase: cdi.Succeeded,
				},
			}

			convertJob := createFakeConvertJob(qcow2PVC, pvcNamespace)

			convertJob.Status.Succeeded = 0
			convertJob.Status.Failed = 3

			converter = createFakeConverter(qcow2PVC, convertJob, dv)

			_, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).To(HaveOccurred())

			// Check if scratch DV is removed
			err = converter.Destination.Client.Get(context.TODO(), types.NamespacedName{Name: dv.Name, Namespace: dv.Namespace}, dv)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createFakeConverter(objects ...runtime.Object) *Converter {
	scheme := runtime.NewScheme()
	_ = cdi.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)

	objs := []runtime.Object{}
	objs = append(objs, objects...)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	return &Converter{
		Destination: &plancontext.Destination{
			Client: client,
		},
		Log:    converterLog,
		Labels: map[string]string{},
	}
}

func createFakeConvertJob(pvc *v1.PersistentVolumeClaim, pvcNamespace string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: meta.ObjectMeta{
			Name:      getJobName(pvc, "convert"),
			Namespace: pvcNamespace,
			Labels: map[string]string{
				base.AnnConversionSourcePVC: pvc.Name,
			},
		},
	}
}
