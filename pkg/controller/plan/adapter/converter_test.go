package adapter

import (
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

		scratchPVC := &v1.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      getScratchPVCName(qcow2PVC),
				Namespace: pvcNamespace,
			},
		}
		scratchPVC.Status.Phase = v1.ClaimBound

		convertJob := &batchv1.Job{
			ObjectMeta: meta.ObjectMeta{
				Name:      getJobName(qcow2PVC, "convert"),
				Namespace: pvcNamespace,
			},
		}

		srcFormatFn := func(pvc *v1.PersistentVolumeClaim) string {
			return pvc.Annotations[base.AnnSourceFormat]
		}

		It("Should not be ready if job is not ready", func() {
			converter = createFakeConverter(qcow2PVC, scratchPVC, convertJob)
			ready, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(ready).To(BeFalse())
		})

		It("Should be ready if job is ready", func() {
			convertJob.Status.Conditions = append(convertJob.Status.Conditions, batchv1.JobCondition{
				Type: batchv1.JobComplete,
			})
			converter = createFakeConverter(qcow2PVC, scratchPVC, convertJob)
			ready, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(ready).To(BeTrue())
		})

		It("Should create job if it does not exist", func() {
			converter = createFakeConverter(qcow2PVC, scratchPVC)
			job, err := converter.ensureJob(qcow2PVC, srcFormatFn(qcow2PVC), "raw")
			Expect(err).ToNot(HaveOccurred())
			Expect(job).ToNot(BeNil())
		})

		It("Should create scratch PVC if it does not exist", func() {
			converter = createFakeConverter(qcow2PVC)
			scratchPVC, err := converter.ensureScratchPVC(qcow2PVC)
			Expect(err).ToNot(HaveOccurred())
			Expect(scratchPVC).ToNot(BeNil())
		})

		It("Should return error if PVC is lost", func() {
			scratchPVC.Status.Phase = v1.ClaimLost
			converter = createFakeConverter(qcow2PVC, scratchPVC)
			ready, err := converter.ConvertPVCs([]*v1.PersistentVolumeClaim{qcow2PVC}, srcFormatFn, "raw")
			Expect(err).To(HaveOccurred())
			Expect(ready).To(BeFalse())
		})
	})
})

func createFakeConverter(objects ...runtime.Object) *Converter {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	client := fake.NewClientBuilder().
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
