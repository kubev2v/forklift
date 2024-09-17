package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/konveyor/forklift-controller/virt-v2v/pkg/customize"
	"github.com/konveyor/forklift-controller/virt-v2v/pkg/global"
	"github.com/konveyor/forklift-controller/virt-v2v/pkg/server"
	"github.com/konveyor/forklift-controller/virt-v2v/pkg/utils"
)

func main() {
	var err error
	if err = virtV2VPrepEnvironment(); err != nil {
		return
	}

	// virt-v2v or virt-v2v-in-place
	if _, found := os.LookupEnv("V2V_inPlace"); found {
		err = runVirtV2vInPlace()
	} else {
		err = runVirtV2v()
	}
	if err != nil {
		fmt.Println("Failed to execute virt-v2v command:", err)
		os.Exit(1)
	}

	// virt-v2v-inspector
	var disks []string
	disks, err = utils.GetLinkedDisks()
	if err != nil {
		fmt.Println("Failed to get linked disk", err)
		os.Exit(1)
	}

	err = runVirtV2VInspection(disks)
	if err != nil {
		fmt.Println("Failed to inspect the disk", err)
		os.Exit(1)
	}
	inspection, err := utils.GetInspectionV2vFromFile(global.INSPECTION)
	if err != nil {
		return
	}
	// virt-customize
	err = customize.Run(disks, inspection.OS.Osinfo)
	if err != nil {
		fmt.Println("Error to customize the VM:", err)
		os.Exit(1)
	}

	err = server.Start()
	if err != nil {
		fmt.Println("Failed to run the server", err)
		os.Exit(1)
	}
}

func runVirtV2VInspection(disks []string) error {
	args := []string{"-v", "-x", "-if", "raw", "-i", "disk", "-O", global.INSPECTION}
	args = append(args, disks...)
	fmt.Println("Running the virt-v2v-inspector with args: ", args)
	v2vCmd := exec.Command("virt-v2v-inspector", args...)
	v2vCmd.Stdout = os.Stdout
	v2vCmd.Stderr = os.Stderr
	return v2vCmd.Run()
}

func runVirtV2vInPlace() error {
	args := []string{"-v", "-x", "-i", "libvirtxml"}
	args = append(args, "--root")
	if val, found := os.LookupEnv("V2V_RootDisk"); found {
		args = append(args, val)
	} else {
		args = append(args, "first")
	}
	args = append(args, "/mnt/v2v/input.xml")
	v2vCmd := exec.Command("/usr/libexec/virt-v2v-in-place", args...)
	v2vCmd.Stdout = os.Stdout
	v2vCmd.Stderr = os.Stderr
	return v2vCmd.Run()
}

func virtV2vBuildCommand() (args []string, err error) {
	args = []string{"-v", "-x"}
	source := os.Getenv("V2V_source")

	requiredEnvVars := map[string][]string{
		global.VSPHERE: {"V2V_libvirtURL", "V2V_secretKey", "V2V_vmName"},
		global.OVA:     {"V2V_diskPath", "V2V_vmName"},
	}

	if envVars, ok := requiredEnvVars[source]; ok {
		if !utils.CheckEnvVariablesSet(envVars...) {
			return nil, fmt.Errorf("Following environment variables need to be defined: %v\n", envVars)
		}
	} else {
		providers := make([]string, len(requiredEnvVars))
		for key := range requiredEnvVars {
			providers = append(providers, key)
		}
		return nil, fmt.Errorf("virt-v2v supports the following providers: {%v}. Provided: %s\n", strings.Join(providers, ", "), source)
	}
	args = append(args, "-o", "local", "-os", global.DIR)

	switch source {
	case global.VSPHERE:
		vsphereArgs, err := virtV2vVsphereArgs()
		if err != nil {
			return nil, err
		}
		args = append(args, vsphereArgs...)
	case global.OVA:
		args = append(args, "-i", "ova", os.Getenv("V2V_diskPath"))
	}

	return args, nil
}

func virtV2vVsphereArgs() (args []string, err error) {
	args = append(args, "--root")
	if utils.CheckEnvVariablesSet("V2V_RootDisk") {
		args = append(args, os.Getenv("V2V_RootDisk"))
	} else {
		args = append(args, "first")
	}
	args = append(args, "-i", "libvirt", "-ic", os.Getenv("V2V_libvirtURL"))
	args = append(args, "-ip", "/etc/secret/secretKey")

	if envStaticIPs := os.Getenv("V2V_staticIPs"); envStaticIPs != "" {
		for _, macToIp := range strings.Split(envStaticIPs, "_") {
			args = append(args, "--mac", macToIp)
		}
	}

	// Adds LUKS keys, if they exist
	luksArgs, err := utils.AddLUKSKeys()
	if err != nil {
		return nil, fmt.Errorf("Error adding LUKS keys: %v", err)
	}
	args = append(args, luksArgs...)

	if info, err := os.Stat(global.VDDK); err == nil && info.IsDir() {
		args = append(args,
			"-it", "vddk",
			"-io", fmt.Sprintf("vddk-libdir=%s", global.VDDK),
			"-io", fmt.Sprintf("vddk-thumbprint=%s", os.Getenv("V2V_fingerprint")),
		)
	}
	var extraArgs []string
	if envExtraArgs := os.Getenv("V2V_extra_args"); envExtraArgs != "" {
		if err := json.Unmarshal([]byte(envExtraArgs), &extraArgs); err != nil {
			return nil, fmt.Errorf("Error parsing extra arguments %v", err)
		}
	}
	args = append(args, extraArgs...)

	args = append(args, "--", os.Getenv("V2V_vmName"))
	return args, nil
}

func runVirtV2v() error {
	args, err := virtV2vBuildCommand()
	if err != nil {
		return err
	}
	v2vCmd := exec.Command("virt-v2v", args...)
	// The virt-v2v-monitor reads the virt-v2v stdout and processes it and exposes the progress of the migration.
	monitorCmd := exec.Command("/usr/local/bin/virt-v2v-monitor")
	monitorCmd.Stdout = os.Stdout
	monitorCmd.Stderr = os.Stderr

	var writer *io.PipeWriter
	monitorCmd.Stdin, writer = io.Pipe()
	v2vCmd.Stdout = writer
	v2vCmd.Stderr = writer
	defer writer.Close()

	if err := monitorCmd.Start(); err != nil {
		fmt.Printf("Error executing monitor command: %v\n", err)
		return err
	}

	fmt.Println("exec:", v2vCmd)
	if err := v2vCmd.Run(); err != nil {
		fmt.Printf("Error executing v2v command: %v\n", err)
		return err
	}

	// virt-v2v is done, we can close the pipe to virt-v2v-monitor
	writer.Close()

	if err := monitorCmd.Wait(); err != nil {
		fmt.Printf("Error waiting for virt-v2v-monitor to finish: %v\n", err)
		return err
	}

	return nil
}

// VirtV2VPrepEnvironment used in the cold migration.
// It creates a links between the downloaded guest image from virt-v2v and mounted PVC.
func virtV2VPrepEnvironment() (err error) {
	source := os.Getenv("V2V_source")
	_, inplace := os.LookupEnv("V2V_inPlace")
	if source == global.VSPHERE && !inplace {
		if _, err := os.Stat("/etc/secret/cacert"); err == nil {
			// use the specified certificate
			err = os.Symlink("/etc/secret/cacert", "/opt/ca-bundle.crt")
			if err != nil {
				fmt.Println("Error creating ca cert link ", err)
				os.Exit(1)
			}
		} else {
			// otherwise, keep system pool certificates
			err := os.Symlink("/etc/pki/tls/certs/ca-bundle.crt.bak", "/opt/ca-bundle.crt")
			if err != nil {
				fmt.Println("Error creating ca cert link ", err)
				os.Exit(1)
			}
		}
	}
	if err = os.MkdirAll(global.DIR, os.ModePerm); err != nil {
		return fmt.Errorf("Error creating directory: %v", err)
	}

	//Disks on Filesystem storage.
	if err = utils.LinkDisks(global.FS); err != nil {
		return
	}
	//Disks on block storage.
	if err = utils.LinkDisks(global.BLOCK); err != nil {
		return
	}
	return nil
}
