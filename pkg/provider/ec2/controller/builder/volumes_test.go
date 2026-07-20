package builder

import (
	"fmt"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
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

	It("should use PVC name template with GenerateName (default)", func() {
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.GenerateName).To(Equal("test-plan-test-vm-disk-0-"))
		Expect(pvc.Name).To(BeEmpty())
	})

	It("should use custom PVC name template from plan", func() {
		builder.Plan.Spec.PVCNameTemplate = "{{.PlanName}}-{{.VmId}}-disk-{{.DiskIndex}}"
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 1)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.GenerateName).To(Equal("test-plan-i-123-disk-1-"))
	})

	It("should use exact name when UseGenerateName is false", func() {
		builder.Plan.Spec.PVCNameTemplateUseGenerateName = false
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.Name).To(Equal("test-plan-test-vm-disk-0"))
		Expect(pvc.GenerateName).To(BeEmpty())
	})

	It("should include EC2-specific VolumeID and SnapshotID in templates", func() {
		builder.Plan.Spec.PVCNameTemplate = "{{trunc 10 .VolumeID}}-{{.DiskIndex}}"
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.GenerateName).To(Equal("vol-0origi-0-"))
	})

	It("should use VM-level template override", func() {
		builder.Plan.Spec.PVCNameTemplate = "plan-{{.DiskIndex}}"
		builder.Plan.Spec.VMs = []planapi.VM{
			{
				Ref:             ref.Ref{ID: "i-123", Name: "test-vm"},
				PVCNameTemplate: "vm-{{.TargetVmName}}-{{.DiskIndex}}",
			},
		}
		pvc, err := builder.BuildDirectPVC(vmRef, volumeInfo, 0)
		Expect(err).NotTo(HaveOccurred())
		Expect(pvc.GenerateName).To(Equal("vm-test-vm-0-"))
	})
})
