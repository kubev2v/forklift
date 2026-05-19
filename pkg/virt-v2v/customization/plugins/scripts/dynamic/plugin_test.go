package dynamic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDynamicPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scripts/Dynamic Plugin Suite")
}

var _ = Describe("Plugin", func() {
	var p *Plugin

	BeforeEach(func() {
		p = &Plugin{}
	})

	Describe("Applicable", func() {
		It("returns true when dynamic scripts dir exists", func() {
			tmpDir := GinkgoT().TempDir()
			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}
			Expect(p.Applicable(ctx)).To(BeTrue())
		})

		It("returns false when dynamic scripts dir does not exist", func() {
			tmp := GinkgoT().TempDir()
			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux}},
				Config:     &config.AppConfig{DynamicScriptsDir: filepath.Join(tmp, "missing")},
				FileSystem: &utils.FileSystemImpl{},
			}
			Expect(p.Applicable(ctx)).To(BeFalse())
		})
	})

	Describe("Apply", func() {
		It("adds Linux run and firstboot scripts", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/001_linux_run_myscript.sh", []byte("#!/bin/bash"), 0755)).To(Succeed())
			Expect(os.WriteFile(tmpDir+"/002_linux_firstboot_init.sh", []byte("#!/bin/bash"), 0755)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Execs).To(HaveLen(2))
			hasRun := false
			hasFirstboot := false
			for _, e := range actions.Execs {
				if e.Type == api.ActionRun {
					hasRun = true
				}
				if e.Type == api.ActionFirstboot {
					hasFirstboot = true
				}
			}
			Expect(hasRun).To(BeTrue())
			Expect(hasFirstboot).To(BeTrue())
		})

		It("adds Windows firstboot scripts as file uploads", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/001_win_firstboot_myscript.ps1", []byte("echo hi"), 0755)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			Expect(actions.Files[0].Type).To(Equal(api.ActionUpload))
		})

		It("ignores files that don't match naming convention", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/random_file.txt", []byte("data"), 0644)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Execs).To(BeEmpty())
			Expect(actions.Files).To(BeEmpty())
		})

		It("ignores directories inside scripts dir", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.Mkdir(tmpDir+"/001_linux_run_subdir.sh", 0755)).To(Succeed())
			Expect(os.WriteFile(tmpDir+"/002_linux_run_real.sh", []byte("#!/bin/bash"), 0755)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Execs).To(HaveLen(1))
		})

		It("Linux ignores Windows scripts", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/001_win_firstboot_setup.ps1", []byte("echo hi"), 0755)).To(Succeed())
			Expect(os.WriteFile(tmpDir+"/002_linux_run_myscript.sh", []byte("#!/bin/bash"), 0755)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Execs).To(HaveLen(1))
			Expect(actions.Files).To(BeEmpty())
		})

		It("Windows ignores Linux scripts", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/001_linux_run_myscript.sh", []byte("#!/bin/bash"), 0755)).To(Succeed())
			Expect(os.WriteFile(tmpDir+"/002_win_firstboot_setup.ps1", []byte("echo hi"), 0755)).To(Succeed())

			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}

			actions, err := p.Apply(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(HaveLen(1))
			Expect(actions.Execs).To(BeEmpty())
		})

		It("returns error when ReadDir fails", func() {
			tmp := GinkgoT().TempDir()
			ctx := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: filepath.Join(tmp, "missing")},
				FileSystem: &utils.FileSystemImpl{},
			}

			_, err := p.Apply(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to read scripts directory"))
		})

		It("ignores files with malformed extensions (missing dot)", func() {
			tmpDir := GinkgoT().TempDir()
			Expect(os.WriteFile(tmpDir+"/001_linux_run_scriptXsh", []byte("#!/bin/bash"), 0755)).To(Succeed())
			Expect(os.WriteFile(tmpDir+"/001_win_firstboot_scriptXps1", []byte("echo hi"), 0755)).To(Succeed())

			ctxLinux := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyLinux, Distro: "rhel"}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}
			actions, err := p.Apply(ctxLinux)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Execs).To(BeEmpty())
			Expect(actions.Files).To(BeEmpty())

			ctxWin := &api.Context{
				Guest:      &api.GuestInfo{OS: api.GuestOS{Family: api.OSFamilyWindows}},
				Config:     &config.AppConfig{DynamicScriptsDir: tmpDir},
				FileSystem: &utils.FileSystemImpl{},
			}
			actions, err = p.Apply(ctxWin)
			Expect(err).NotTo(HaveOccurred())
			Expect(actions.Files).To(BeEmpty())
			Expect(actions.Execs).To(BeEmpty())
		})
	})
})
