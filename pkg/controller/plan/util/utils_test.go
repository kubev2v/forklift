package util

import (
	"math"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Helper function to check if VM name is a valid DNS1123 subdomain
func validateVmName(name string) bool {
	return len(validation.IsDNS1123Subdomain(name)) == 0
}

var _ = Describe("Plan/utils", func() {
	DescribeTable("convert dev", func(dev string, number int) {
		Expect(GetDeviceNumber(dev)).Should(Equal(number))
	},
		Entry("sda", "/dev/sda", 1),
		Entry("sdb", "/dev/sdb", 2),
		Entry("sdz", "/dev/sdz", 26),
		Entry("sda1", "/dev/sda1", 1),
		Entry("sda5", "/dev/sda5", 1),
		Entry("sdb2", "/dev/sdb2", 2),
		Entry("sdza", "/dev/sdza", 26),
		Entry("sdzb", "/dev/sdzb", 26),
		Entry("sd", "/dev/sd", 0),
		Entry("test", "test", 0),
	)

	Context("VM Name Handler", func() {
		It("should handle all cases in name adjustments", func() {
			originalVmName := "----------------Vm!@#$%^&*()_+-Name/.is,';[]-CorREct-<>123----------------------"
			newVmName := "vm-name.is-correct-123"
			changedName := ChangeVmName(originalVmName)
			Expect(changedName).To(Equal(newVmName))
			Expect(validateVmName(changedName)).To(BeTrue(), "Changed name should match DNS1123 subdomain format")
		})

		It("should handle the case that the VM name is empty after all removals", func() {
			emptyVM := ".__."
			newVmNameFromId := "vm-"
			changedEmptyName := ChangeVmName(emptyVM)
			Expect(changedEmptyName).To(ContainSubstring(newVmNameFromId))
			Expect(validateVmName(changedEmptyName)).To(BeTrue(), "Changed name from empty should match DNS1123 subdomain format")
		})

		It("should handle multiple consecutive dots", func() {
			multiDotVM := "mtv.func.-.rhel.-...8.8"
			expectedMultiDotResult := "mtv.func.rhel.8.8"
			changedMultiDotName := ChangeVmName(multiDotVM)
			Expect(changedMultiDotName).To(Equal(expectedMultiDotResult))
			Expect(validateVmName(changedMultiDotName)).To(BeTrue(), "Changed name with multiple dots should match DNS1123 subdomain format")

			multiDotVM2 := ".....mtv.func..-...............rhel.-...8.8"
			expectedMultiDotResult2 := "mtv.func.rhel.8.8"
			changedMultiDotName2 := ChangeVmName(multiDotVM2)
			Expect(changedMultiDotName2).To(Equal(expectedMultiDotResult2))
			Expect(validateVmName(changedMultiDotName2)).To(BeTrue(), "Changed name with multiple leading dots should match DNS1123 subdomain format")
		})

		It("should convert spaces to dashes", func() {
			spaceVM := "vm with spaces in name"
			expectedSpaceResult := "vm-with-spaces-in-name"
			changedSpaceName := ChangeVmName(spaceVM)
			Expect(changedSpaceName).To(Equal(expectedSpaceResult))
			Expect(validateVmName(changedSpaceName)).To(BeTrue(), "Changed name with spaces should match DNS1123 subdomain format")
		})

		It("should convert + signs to dashes", func() {
			plusVM := "vm+with+plus+signs"
			expectedPlusResult := "vm-with-plus-signs"
			changedPlusName := ChangeVmName(plusVM)
			Expect(changedPlusName).To(Equal(expectedPlusResult))
			Expect(validateVmName(changedPlusName)).To(BeTrue(), "Changed name with plus signs should match DNS1123 subdomain format")
		})

		It("should remove multiple consecutive dashes", func() {
			multipleDashVM := "vm---with----multiple-----dashes"
			expectedMultipleDashResult := "vm-with-multiple-dashes"
			changedMultipleDashName := ChangeVmName(multipleDashVM)
			Expect(changedMultipleDashName).To(Equal(expectedMultipleDashResult))
			Expect(validateVmName(changedMultipleDashName)).To(BeTrue(), "Changed name with multiple dashes should match DNS1123 subdomain format")
		})

		It("should handle complex case with spaces, plus signs, and multiple dashes", func() {
			complexVM := "vm   +++with   ---mixed+++   ---characters"
			expectedComplexResult := "vm-with-mixed-characters"
			changedComplexName := ChangeVmName(complexVM)
			Expect(changedComplexName).To(Equal(expectedComplexResult))
			Expect(validateVmName(changedComplexName)).To(BeTrue(), "Changed name with mixed special characters should match DNS1123 subdomain format")
		})

		It("should convert * (asterisk) to dashes", func() {
			asteriskVM := "vm*with*asterisk*characters"
			expectedAsteriskResult := "vm-with-asterisk-characters"
			changedAsteriskName := ChangeVmName(asteriskVM)
			Expect(changedAsteriskName).To(Equal(expectedAsteriskResult))
			Expect(validateVmName(changedAsteriskName)).To(BeTrue(), "Changed name with asterisk should match DNS1123 subdomain format")
		})

		It("should trim names longer than NameMaxLength", func() {
			const labelMax = validation.DNS1123LabelMaxLength

			long := strings.Repeat("a", labelMax+10)
			changed := ChangeVmName(long)
			Expect(changed).To(HaveLen(labelMax))
			Expect(validateVmName(changed)).To(BeTrue())
		})
	})
})

func TestIsNetAppShiftPersistentVolumeClaim(t *testing.T) {
	t.Parallel()
	if IsNetAppShiftPersistentVolumeClaim(nil) {
		t.Fatal("nil")
	}
	if IsNetAppShiftPersistentVolumeClaim(map[string]string{AnnNfsServer: "h"}) {
		t.Fatal("partial")
	}
	if !IsNetAppShiftPersistentVolumeClaim(map[string]string{AnnNfsServer: "h", AnnNfsPath: "/p"}) {
		t.Fatal("expected true")
	}
}

func TestCalculateSpaceWithCDIOverhead_NilClient(t *testing.T) {
	t.Parallel()
	_, err := CalculateSpaceWithCDIOverhead(nil, "sc", 1<<30)
	if err == nil {
		t.Fatal("expected error with nil client")
	}
}

func fakeCDIClient(t *testing.T, objs ...runtime.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := cdi.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
}

func inflatedWithOverhead(raw int64, overhead float64) int64 {
	aligned := RoundUp(raw, DefaultAlignBlockSize)
	return int64(math.Ceil(float64(aligned) / (1.0 - overhead)))
}

func TestCalculateSpaceWithCDIOverhead_PerStorageClassSuccess(t *testing.T) {
	t.Parallel()
	const scName = "gold"
	raw := int64(10 * 1024 * 1024)
	const overheadVal = 0.10
	cfg := &cdi.CDIConfig{
		ObjectMeta: metav1.ObjectMeta{Name: cdiConfigName},
		Spec:       cdi.CDIConfigSpec{},
		Status: cdi.CDIConfigStatus{
			FilesystemOverhead: &cdi.FilesystemOverhead{
				StorageClass: map[string]cdi.Percent{
					scName: cdi.Percent("0.10"),
				},
				Global: cdi.Percent("0.20"),
			},
		},
	}
	got, err := CalculateSpaceWithCDIOverhead(fakeCDIClient(t, cfg), scName, raw)
	if err != nil {
		t.Fatal(err)
	}
	want := inflatedWithOverhead(raw, overheadVal)
	if got != want {
		t.Fatalf("got %d want %d (per-SC overhead should win over global)", got, want)
	}
}

func TestCalculateSpaceWithCDIOverhead_PerStorageClassInvalidDoesNotFallBackToGlobal(t *testing.T) {
	t.Parallel()
	const scName = "gold"
	cfg := &cdi.CDIConfig{
		ObjectMeta: metav1.ObjectMeta{Name: cdiConfigName},
		Spec:       cdi.CDIConfigSpec{},
		Status: cdi.CDIConfigStatus{
			FilesystemOverhead: &cdi.FilesystemOverhead{
				StorageClass: map[string]cdi.Percent{
					scName: cdi.Percent("not-a-float"),
				},
				Global: cdi.Percent("0.08"),
			},
		},
	}
	_, err := CalculateSpaceWithCDIOverhead(fakeCDIClient(t, cfg), scName, 1<<20)
	if err == nil {
		t.Fatal("expected error when per-SC value is unparseable")
	}
	// Must not silently use global (0.08) or default overhead.
	if !strings.Contains(err.Error(), scName) {
		t.Fatalf("expected error to name storage class: %v", err)
	}
}

func TestCalculateSpaceWithCDIOverhead_GlobalOnly(t *testing.T) {
	t.Parallel()
	raw := int64(5 * 1024 * 1024)
	const overheadVal = 0.08
	cfg := &cdi.CDIConfig{
		ObjectMeta: metav1.ObjectMeta{Name: cdiConfigName},
		Spec:       cdi.CDIConfigSpec{},
		Status: cdi.CDIConfigStatus{
			FilesystemOverhead: &cdi.FilesystemOverhead{
				Global: cdi.Percent("0.08"),
			},
		},
	}
	got, err := CalculateSpaceWithCDIOverhead(fakeCDIClient(t, cfg), "no-entry-for-this-sc", raw)
	if err != nil {
		t.Fatal(err)
	}
	want := inflatedWithOverhead(raw, overheadVal)
	if got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestCalculateSpaceWithCDIOverhead_MissingCDIConfig(t *testing.T) {
	t.Parallel()
	_, err := CalculateSpaceWithCDIOverhead(fakeCDIClient(t), "sc", 1<<30)
	if err == nil {
		t.Fatal("expected error when CDIConfig is missing")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestCalculateSpaceWithCDIOverhead_GlobalInvalidReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &cdi.CDIConfig{
		ObjectMeta: metav1.ObjectMeta{Name: cdiConfigName},
		Spec:       cdi.CDIConfigSpec{},
		Status: cdi.CDIConfigStatus{
			FilesystemOverhead: &cdi.FilesystemOverhead{
				Global: cdi.Percent("bad"),
			},
		},
	}
	_, err := CalculateSpaceWithCDIOverhead(fakeCDIClient(t, cfg), "any-sc", 1<<20)
	if err == nil {
		t.Fatal("expected error when global overhead is invalid")
	}
	if !strings.Contains(err.Error(), "global") {
		t.Fatalf("expected global overhead in error: %v", err)
	}
}
