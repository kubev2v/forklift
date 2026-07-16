package builder

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/provider/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var _ = Describe("BuildDirectPVC", func() {
	var (
		builder    *Builder
		vmRef      ref.Ref
		volumeInfo *VolumeInfo
	)

	BeforeEach(func() {
		ctx := testutil.NewContextBuilder().Build()
		ctx.Labeler = plancontext.Labeler{Context: ctx}
		builder = New(ctx)
		vmRef = ref.Ref{ID: "i-123", Name: "test-vm"}
		volumeInfo = &VolumeInfo{
			EBSVolumeID:      "vol-0abc123",
			OriginalVolumeID: "vol-0original",
			SnapshotID:       "snap-0abc123",
			SizeGiB:          10,
			VolumeType:       "gp3",
			AvailabilityZone: "us-east-1a",
		}
	})

	It("should set AnnDiskSource annotation to the original volume ID", func() {
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Annotations).To(HaveKeyWithValue(planbase.AnnDiskSource, volumeInfo.OriginalVolumeID))
	})

	It("should set all expected annotations", func() {
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 2)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Annotations).To(HaveKeyWithValue("forklift.konveyor.io/original-volume-id", "vol-0original"))
		Expect(pvc.Annotations).To(HaveKeyWithValue("forklift.konveyor.io/ebs-volume-id", "vol-0abc123"))
		Expect(pvc.Annotations).To(HaveKeyWithValue("forklift.konveyor.io/snapshot-id", "snap-0abc123"))
		Expect(pvc.Annotations).To(HaveKeyWithValue("forklift.konveyor.io/disk-index", fmt.Sprintf("%d", 2)))
	})

	It("should set volume-id label", func() {
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Labels).To(HaveKeyWithValue("forklift.konveyor.io/volume-id", "vol-0original"))
	})

	It("should use block volume mode", func() {
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Spec.VolumeMode).NotTo(BeNil())
		Expect(*pvc.Spec.VolumeMode).To(Equal(core.PersistentVolumeBlock))
	})
})
