package vsphere

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("inspectionparser", func() {
	Describe("GetRootDeviceFromConfig", func() {
		It("extracts root device from valid XML", func() {
			xmlData := `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <operatingsystem>
    <name>linux</name>
    <distro>fedora</distro>
    <osinfo>fedora32</osinfo>
    <arch>x86_64</arch>
    <root>/dev/sda1</root>
  </operatingsystem>
</v2v-inspection>`
			root, err := GetRootDeviceFromConfig(xmlData)
			Expect(err).ToNot(HaveOccurred())
			Expect(root).To(Equal("/dev/sda1"))
		})

		It("returns empty string when root element is absent", func() {
			xmlData := `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <operatingsystem>
    <name>linux</name>
    <distro>fedora</distro>
    <osinfo>fedora32</osinfo>
    <arch>x86_64</arch>
  </operatingsystem>
</v2v-inspection>`
			root, err := GetRootDeviceFromConfig(xmlData)
			Expect(err).ToNot(HaveOccurred())
			Expect(root).To(BeEmpty())
		})

		It("handles Windows root device path", func() {
			xmlData := `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <operatingsystem>
    <name>windows</name>
    <osinfo>win10</osinfo>
    <arch>x86_64</arch>
    <root>/dev/sda2</root>
  </operatingsystem>
</v2v-inspection>`
			root, err := GetRootDeviceFromConfig(xmlData)
			Expect(err).ToNot(HaveOccurred())
			Expect(root).To(Equal("/dev/sda2"))
		})

		It("returns error for invalid XML", func() {
			_, err := GetRootDeviceFromConfig("<invalid>")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ParseInspectionFromString with root field", func() {
		It("populates the Root field", func() {
			xmlData := `<v2v-inspection>
  <operatingsystem>
    <name>linux</name>
    <distro>rhel</distro>
    <osinfo>rhel9</osinfo>
    <arch>x86_64</arch>
    <root>/dev/sdb1</root>
  </operatingsystem>
</v2v-inspection>`
			result, err := ParseInspectionFromString(xmlData)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.OS.Root).To(Equal("/dev/sdb1"))
			Expect(result.OS.Distro).To(Equal("rhel"))
		})
	})
})
