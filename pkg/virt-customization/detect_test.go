package customization_test

import (
	"os"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	customization "github.com/kubev2v/forklift/pkg/virt-customization"
	"github.com/kubev2v/forklift/pkg/virt-customization/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeFS struct{}

func (f *fakeFS) Symlink(_, _ string) error                         { return nil }
func (f *fakeFS) Stat(_ string) (os.FileInfo, error)                { return nil, os.ErrNotExist }
func (f *fakeFS) WriteFile(_ string, _ []byte, _ os.FileMode) error { return nil }
func (f *fakeFS) ReadDir(_ string) ([]os.DirEntry, error)           { return nil, os.ErrNotExist }

func TestPostprocess(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Customize Suite")
}

var _ = Describe("AllPlugins", func() {
	It("returns exactly one plugin (example/hello)", func() {
		plugins := customization.AllPlugins()
		Expect(plugins).To(HaveLen(1))
		Expect(plugins[0].Name()).To(Equal("example/hello"))
	})
})

var _ = Describe("Resolve", func() {
	It("returns empty when no plugins are applicable", func() {
		ctx := &api.Context{
			Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
			FileSystem: &fakeFS{},
			Config:     &config.AppConfig{},
		}
		plugins := customization.Resolve(ctx)
		Expect(plugins).To(BeEmpty())
	})

	It("returns empty for nil context", func() {
		plugins := customization.Resolve(nil)
		Expect(plugins).To(BeEmpty())
	})
})
