// Generated-by: Claude
package utils

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("XML Reader", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "xml-reader-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("GetInspectionV2vFromFile", func() {
		It("parses valid XML file correctly", func() {
			xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<v2v>
  <operatingsystem>
    <name>Fedora Linux</name>
    <distro>fedora</distro>
    <osinfo>linux</osinfo>
    <arch>x86_64</arch>
  </operatingsystem>
</v2v>`
			xmlPath := filepath.Join(tempDir, "inspection.xml")
			err := os.WriteFile(xmlPath, []byte(xmlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := GetInspectionV2vFromFile(xmlPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.OS.Name).To(Equal("Fedora Linux"))
			Expect(result.OS.Distro).To(Equal("fedora"))
			Expect(result.OS.Osinfo).To(Equal("linux"))
			Expect(result.OS.Arch).To(Equal("x86_64"))
		})

		It("parses Windows OS info correctly", func() {
			xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<v2v>
  <operatingsystem>
    <name>Windows 10</name>
    <distro>windows</distro>
    <osinfo>win10</osinfo>
    <arch>x86_64</arch>
  </operatingsystem>
</v2v>`
			xmlPath := filepath.Join(tempDir, "inspection.xml")
			err := os.WriteFile(xmlPath, []byte(xmlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := GetInspectionV2vFromFile(xmlPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.OS.Name).To(Equal("Windows 10"))
			Expect(result.OS.Osinfo).To(Equal("win10"))
		})

		It("returns error for non-existent file", func() {
			result, err := GetInspectionV2vFromFile("/non/existent/file.xml")
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("returns error for invalid XML", func() {
			xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<v2v>
  <operatingsystem>
    <name>Incomplete XML`
			xmlPath := filepath.Join(tempDir, "invalid.xml")
			err := os.WriteFile(xmlPath, []byte(xmlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := GetInspectionV2vFromFile(xmlPath)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("handles empty operatingsystem section", func() {
			xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<v2v>
  <operatingsystem>
  </operatingsystem>
</v2v>`
			xmlPath := filepath.Join(tempDir, "empty.xml")
			err := os.WriteFile(xmlPath, []byte(xmlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := GetInspectionV2vFromFile(xmlPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.OS.Name).To(BeEmpty())
			Expect(result.OS.Distro).To(BeEmpty())
		})
	})

	Describe("InspectionOS.IsWindows", func() {
		DescribeTable("correctly identifies Windows OS",
			func(osinfo string, expected bool) {
				os := InspectionOS{Osinfo: osinfo}
				Expect(os.IsWindows()).To(Equal(expected))
			},
			Entry("win10", "win10", true),
			Entry("win2k19", "win2k19", true),
			Entry("Windows Server 2019", "Windows Server 2019", true),
			Entry("WIN2K22", "WIN2K22", true),
			Entry("linux", "linux", false),
			Entry("fedora", "fedora", false),
			Entry("rhel8", "rhel8", false),
			Entry("ubuntu", "ubuntu", false),
			Entry("empty string", "", false),
			Entry("centos", "centos", false),
			Entry("freebsd", "freebsd", false),
		)
	})
})
