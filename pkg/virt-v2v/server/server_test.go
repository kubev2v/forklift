// Generated-by: Claude
package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server test suite")
}

var _ = Describe("Server", func() {
	var tempDir string
	var s Server
	var appConfig *config.AppConfig

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "server-test")
		Expect(err).ToNot(HaveOccurred())

		appConfig = &config.AppConfig{
			Workdir:              tempDir,
			InspectionOutputFile: filepath.Join(tempDir, "inspection.xml"),
		}
		s = Server{AppConfig: appConfig}

		// Reset warnings before each test
		warnings = nil
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("vmHandler", func() {
		It("returns YAML content when file exists", func() {
			yamlContent := `apiVersion: v1
kind: VirtualMachine
metadata:
  name: test-vm`
			err := os.WriteFile(filepath.Join(tempDir, "vm.yaml"), []byte(yamlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/vm", nil)
			w := httptest.NewRecorder()

			s.vmHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/yaml"))
			Expect(w.Body.String()).To(Equal(yamlContent))
		})

		It("returns 204 No Content for in-place conversion without YAML", func() {
			appConfig.IsInPlace = true

			req := httptest.NewRequest(http.MethodGet, "/vm", nil)
			w := httptest.NewRecorder()

			s.vmHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("returns 500 when YAML file path is empty and not in-place", func() {
			appConfig.IsInPlace = false

			req := httptest.NewRequest(http.MethodGet, "/vm", nil)
			w := httptest.NewRecorder()

			s.vmHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("returns 500 when YAML file cannot be read", func() {
			// Create a directory instead of a file with .yaml extension
			yamlDir := filepath.Join(tempDir, "invalid.yaml")
			err := os.Mkdir(yamlDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			// Create a valid YAML file so getVmYamlFile finds something
			err = os.WriteFile(filepath.Join(tempDir, "vm.yaml"), []byte("content"), 0000)
			Expect(err).ToNot(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/vm", nil)
			w := httptest.NewRecorder()

			s.vmHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("inspectorHandler", func() {
		It("returns XML content when inspection file exists", func() {
			xmlContent := `<?xml version="1.0"?>
<v2v>
  <operatingsystem>
    <name>Fedora</name>
  </operatingsystem>
</v2v>`
			err := os.WriteFile(appConfig.InspectionOutputFile, []byte(xmlContent), 0644)
			Expect(err).ToNot(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/inspection", nil)
			w := httptest.NewRecorder()

			s.inspectorHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/xml"))
			Expect(w.Body.String()).To(Equal(xmlContent))
		})

		It("returns 500 when inspection file does not exist", func() {
			req := httptest.NewRequest(http.MethodGet, "/inspection", nil)
			w := httptest.NewRecorder()

			s.inspectorHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("warningsHandler", func() {
		It("returns 204 when no warnings", func() {
			req := httptest.NewRequest(http.MethodGet, "/warnings", nil)
			w := httptest.NewRecorder()

			s.warningsHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
		})

		It("returns JSON warnings when warnings exist", func() {
			AddWarning(Warning{
				Reason:  "TestReason",
				Message: "Test message",
			})

			req := httptest.NewRequest(http.MethodGet, "/warnings", nil)
			w := httptest.NewRecorder()

			s.warningsHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(w.Body.String()).To(ContainSubstring("TestReason"))
			Expect(w.Body.String()).To(ContainSubstring("Test message"))
		})

		It("returns multiple warnings", func() {
			AddWarning(Warning{Reason: "Reason1", Message: "Message1"})
			AddWarning(Warning{Reason: "Reason2", Message: "Message2"})

			req := httptest.NewRequest(http.MethodGet, "/warnings", nil)
			w := httptest.NewRecorder()

			s.warningsHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("Reason1"))
			Expect(w.Body.String()).To(ContainSubstring("Reason2"))
		})
	})

	Describe("shutdownHandler", func() {
		It("returns 204 No Content", func() {
			// Create a test server to avoid nil pointer
			server = &http.Server{}

			req := httptest.NewRequest(http.MethodPost, "/shutdown", nil)
			w := httptest.NewRecorder()

			s.shutdownHandler(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})
	})

	Describe("getVmYamlFile", func() {
		It("returns first YAML file in directory", func() {
			err := os.WriteFile(filepath.Join(tempDir, "vm.yaml"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := s.getVmYamlFile(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(filepath.Join(tempDir, "vm.yaml")))
		})

		It("returns error when no YAML files exist", func() {
			result, err := s.getVmYamlFile(tempDir)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("returns first file when multiple YAML files exist", func() {
			err := os.WriteFile(filepath.Join(tempDir, "a.yaml"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "b.yaml"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := s.getVmYamlFile(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeEmpty())
		})

		It("ignores non-YAML files", func() {
			err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("content"), 0644)
			Expect(err).ToNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "config.json"), []byte("{}"), 0644)
			Expect(err).ToNot(HaveOccurred())

			result, err := s.getVmYamlFile(tempDir)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Describe("AddWarning", func() {
		It("adds warning to global warnings slice", func() {
			warnings = nil // Reset
			AddWarning(Warning{Reason: "Test", Message: "Test message"})
			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0].Reason).To(Equal("Test"))
		})

		It("appends multiple warnings", func() {
			warnings = nil // Reset
			AddWarning(Warning{Reason: "Test1", Message: "Message1"})
			AddWarning(Warning{Reason: "Test2", Message: "Message2"})
			Expect(warnings).To(HaveLen(2))
		})
	})
})
