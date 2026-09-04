package settings_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubev2v/forklift/pkg/settings"
)

var _ = Describe("vSphere post-migration tag settings", func() {
	AfterEach(func() {
		_ = os.Unsetenv(settings.VspherePostMigrationTagCategoryEv)
		_ = os.Unsetenv(settings.VspherePostMigrationTagNameEv)
		_ = os.Unsetenv(settings.VspherePostMigrationTaggingEnabledEv)
	})

	It("uses defaults when env vars are unset", func() {
		_ = os.Unsetenv(settings.VspherePostMigrationTagCategoryEv)
		_ = os.Unsetenv(settings.VspherePostMigrationTagNameEv)
		err := settings.Settings.Providers.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Settings.PostMigrationTagCategory).To(Equal(settings.DefaultVspherePostMigrationTagCategory))
		Expect(settings.Settings.PostMigrationTagName).To(Equal(settings.DefaultVspherePostMigrationTagName))
	})

	It("reads VSPHERE_POST_MIGRATION_TAG_CATEGORY", func() {
		Expect(os.Setenv(settings.VspherePostMigrationTagCategoryEv, "MyOrg")).To(Succeed())
		err := settings.Settings.Providers.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Settings.PostMigrationTagCategory).To(Equal("MyOrg"))
	})

	It("reads VSPHERE_POST_MIGRATION_TAG_NAME", func() {
		Expect(os.Setenv(settings.VspherePostMigrationTagNameEv, "done")).To(Succeed())
		err := settings.Settings.Providers.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Settings.PostMigrationTagName).To(Equal("done"))
	})

	It("reads VSPHERE_POST_MIGRATION_TAGGING_ENABLED as false", func() {
		Expect(os.Setenv(settings.VspherePostMigrationTaggingEnabledEv, "false")).To(Succeed())
		err := settings.Settings.Providers.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Settings.PostMigrationTaggingEnabled).To(BeFalse())
	})

	It("defaults VSPHERE_POST_MIGRATION_TAGGING_ENABLED to true", func() {
		_ = os.Unsetenv(settings.VspherePostMigrationTaggingEnabledEv)
		err := settings.Settings.Providers.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Settings.PostMigrationTaggingEnabled).To(BeTrue())
	})
})
