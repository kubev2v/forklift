// Generated-by: Claude
package utils

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command", func() {
	Describe("CommandBuilderImpl", func() {
		var builder *CommandBuilderImpl

		BeforeEach(func() {
			builder = &CommandBuilderImpl{}
		})

		Describe("New", func() {
			It("initializes builder with command name", func() {
				result := builder.New("virt-customize")
				Expect(builder.BaseCommand).To(Equal("virt-customize"))
				Expect(builder.Args).To(BeEmpty())
				Expect(result).To(Equal(builder))
			})

			It("resets args when called again", func() {
				builder.New("first-command")
				builder.AddFlag("--flag")

				builder.New("second-command")
				Expect(builder.BaseCommand).To(Equal("second-command"))
				Expect(builder.Args).To(BeEmpty())
			})
		})

		Describe("AddArg", func() {
			It("adds flag and value to args", func() {
				builder.New("cmd")
				result := builder.AddArg("--output", "/tmp/out")
				Expect(builder.Args).To(Equal([]string{"--output", "/tmp/out"}))
				Expect(result).To(Equal(builder))
			})

			It("skips adding when value is empty", func() {
				builder.New("cmd")
				builder.AddArg("--output", "")
				Expect(builder.Args).To(BeEmpty())
			})

			It("chains multiple AddArg calls", func() {
				builder.New("cmd")
				builder.AddArg("--input", "/in").AddArg("--output", "/out")
				Expect(builder.Args).To(Equal([]string{"--input", "/in", "--output", "/out"}))
			})
		})

		Describe("AddArgs", func() {
			It("adds flag with multiple values", func() {
				builder.New("cmd")
				result := builder.AddArgs("--key", "value1", "value2", "value3")
				Expect(builder.Args).To(Equal([]string{"--key", "value1", "--key", "value2", "--key", "value3"}))
				Expect(result).To(Equal(builder))
			})

			It("skips empty values", func() {
				builder.New("cmd")
				builder.AddArgs("--key", "value1", "", "value3")
				Expect(builder.Args).To(Equal([]string{"--key", "value1", "--key", "value3"}))
			})

			It("handles single value", func() {
				builder.New("cmd")
				builder.AddArgs("--key", "single")
				Expect(builder.Args).To(Equal([]string{"--key", "single"}))
			})

			It("handles no values", func() {
				builder.New("cmd")
				builder.AddArgs("--key")
				Expect(builder.Args).To(BeEmpty())
			})
		})

		Describe("AddFlag", func() {
			It("adds flag without value", func() {
				builder.New("cmd")
				result := builder.AddFlag("--verbose")
				Expect(builder.Args).To(Equal([]string{"--verbose"}))
				Expect(result).To(Equal(builder))
			})

			It("chains multiple flags", func() {
				builder.New("cmd")
				builder.AddFlag("-v").AddFlag("-x").AddFlag("--debug")
				Expect(builder.Args).To(Equal([]string{"-v", "-x", "--debug"}))
			})
		})

		Describe("AddPositional", func() {
			It("adds positional argument", func() {
				builder.New("cmd")
				result := builder.AddPositional("/path/to/file")
				Expect(builder.Args).To(Equal([]string{"/path/to/file"}))
				Expect(result).To(Equal(builder))
			})

			It("skips empty positional", func() {
				builder.New("cmd")
				builder.AddPositional("")
				Expect(builder.Args).To(BeEmpty())
			})

			It("adds multiple positionals", func() {
				builder.New("cmd")
				builder.AddPositional("--").AddPositional("vm-name")
				Expect(builder.Args).To(Equal([]string{"--", "vm-name"}))
			})
		})

		Describe("AddExtraArgs", func() {
			It("adds extra arguments as-is", func() {
				builder.New("cmd")
				result := builder.AddExtraArgs("--custom", "value", "--another")
				Expect(builder.Args).To(Equal([]string{"--custom", "value", "--another"}))
				Expect(result).To(Equal(builder))
			})

			It("handles no extra args", func() {
				builder.New("cmd")
				builder.AddExtraArgs()
				Expect(builder.Args).To(BeEmpty())
			})
		})

		Describe("Build", func() {
			It("creates CommandExecutor", func() {
				builder.New("echo")
				builder.AddPositional("hello")

				executor := builder.Build()
				Expect(executor).ToNot(BeNil())
			})
		})

		Describe("Full command building scenario", func() {
			It("builds complete virt-customize command", func() {
				builder.New("virt-customize")
				builder.AddFlag("--verbose").
					AddFlag("-x").
					AddArg("--format", "raw").
					AddArg("--add", "/var/tmp/v2v/disk1").
					AddArg("--add", "/var/tmp/v2v/disk2").
					AddArg("--run", "/tmp/script.sh").
					AddArg("--firstboot", "/tmp/firstboot.sh")

				Expect(builder.BaseCommand).To(Equal("virt-customize"))
				Expect(builder.Args).To(Equal([]string{
					"--verbose",
					"-x",
					"--format", "raw",
					"--add", "/var/tmp/v2v/disk1",
					"--add", "/var/tmp/v2v/disk2",
					"--run", "/tmp/script.sh",
					"--firstboot", "/tmp/firstboot.sh",
				}))
			})

			It("builds complete virt-v2v command", func() {
				builder.New("virt-v2v")
				builder.AddFlag("-v").
					AddFlag("-x").
					AddArg("-o", "kubevirt").
					AddArg("-os", "/var/tmp/v2v").
					AddArg("-on", "new-vm-name").
					AddArg("-i", "libvirt").
					AddArg("-ic", "vpx://user@vcenter/Datacenter/host/esxi").
					AddPositional("--").
					AddPositional("original-vm-name")

				Expect(builder.BaseCommand).To(Equal("virt-v2v"))
				Expect(builder.Args).To(Equal([]string{
					"-v",
					"-x",
					"-o", "kubevirt",
					"-os", "/var/tmp/v2v",
					"-on", "new-vm-name",
					"-i", "libvirt",
					"-ic", "vpx://user@vcenter/Datacenter/host/esxi",
					"--",
					"original-vm-name",
				}))
			})
		})
	})

	Describe("Command", func() {
		Describe("SetStdout", func() {
			It("sets stdout writer", func() {
				builder := &CommandBuilderImpl{}
				builder.New("echo")
				builder.AddPositional("test")

				cmd := builder.Build().(*Command)
				var buf bytes.Buffer
				cmd.SetStdout(&buf)

				Expect(cmd.cmd.Stdout).To(Equal(&buf))
			})
		})

		Describe("SetStderr", func() {
			It("sets stderr writer", func() {
				builder := &CommandBuilderImpl{}
				builder.New("echo")
				builder.AddPositional("test")

				cmd := builder.Build().(*Command)
				var buf bytes.Buffer
				cmd.SetStderr(&buf)

				Expect(cmd.cmd.Stderr).To(Equal(&buf))
			})
		})

		Describe("SetStdin", func() {
			It("sets stdin reader", func() {
				builder := &CommandBuilderImpl{}
				builder.New("cat")

				cmd := builder.Build().(*Command)
				var buf bytes.Buffer
				cmd.SetStdin(&buf)

				Expect(cmd.cmd.Stdin).To(Equal(&buf))
			})
		})

		Describe("Run", func() {
			It("executes command successfully", func() {
				builder := &CommandBuilderImpl{}
				builder.New("true") // Command that always succeeds

				cmd := builder.Build()
				err := cmd.Run()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns error for failed command", func() {
				builder := &CommandBuilderImpl{}
				builder.New("false") // Command that always fails

				cmd := builder.Build()
				err := cmd.Run()
				Expect(err).To(HaveOccurred())
			})

			It("returns error for non-existent command", func() {
				builder := &CommandBuilderImpl{}
				builder.New("nonexistent-command-12345")

				cmd := builder.Build()
				err := cmd.Run()
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("Start and Wait", func() {
			It("starts and waits for command", func() {
				builder := &CommandBuilderImpl{}
				builder.New("true")

				cmd := builder.Build()
				err := cmd.Start()
				Expect(err).ToNot(HaveOccurred())

				err = cmd.Wait()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns error for failed command on wait", func() {
				builder := &CommandBuilderImpl{}
				builder.New("false")

				cmd := builder.Build()
				err := cmd.Start()
				Expect(err).ToNot(HaveOccurred())

				err = cmd.Wait()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
