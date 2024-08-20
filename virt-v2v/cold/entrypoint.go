package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	OVA     = "ova"
	VSPHERE = "vSphere"
	DIR     = "/var/tmp/v2v"
	FS      = "/mnt/disks/disk[0-9]*"
	BLOCK   = "/dev/block[0-9]*"
	VDDK    = "/opt/vmware-vix-disklib-distrib"
	LUKSDIR = "/etc/luks"
)

var (
	xmlFilePath string
	server      *http.Server
)

const LETTERS = "abcdefghijklmnopqrstuvwxyz"
const LETTERS_LENGTH = len(LETTERS)

func main() {
	var err error
	if _, found := os.LookupEnv("V2V_inPlace"); found {
		err = convertVirtV2vInPlace()
	} else {
		err = convertVirtV2v()
	}
	if err != nil {
		fmt.Println("Error executing virt-v2v command ", err)
		os.Exit(1)
	}
}

func getVmDiskPaths(domain *OvaVmconfig) []string {
	var resp []string
	for _, disk := range domain.Devices.Disks {
		if disk.Source.File != "" {
			resp = append(resp, disk.Source.File)
		}
	}
	return resp
}

func customizeVM(source string, xmlFilePath string) error {
	domain, err := GetDomainFromXml(xmlFilePath)
	if err != nil {
		fmt.Printf("Error mapping xml to domain: %v\n", err)

		// No customization if we can't parse virt-v2v output.
		return err
	}

	// Get operating system.
	operatingSystem := domain.Metadata.LibOsInfo.V2VOS.ID
	if operatingSystem == "" {
		fmt.Printf("Warning: no operating system found")

		// No customization when no known OS detected.
		return nil
	} else {
		fmt.Printf("Operating System ID: %s\n", operatingSystem)
	}

	// Get domain disks.
	disks := getVmDiskPaths(domain)
	if len(disks) == 0 {
		fmt.Printf("Warning: no V2V domain disks found")

		// No customization when no disks found.
		return nil
	} else {
		fmt.Printf("V2V domain disks: %v\n", disks)
	}

	// Customization for vSphere source.
	if source == VSPHERE {
		// Windows
		if strings.Contains(operatingSystem, "win") {
			t := EmbedTool{filesystem: &scriptFS}

			err = CustomizeWindows(disks, DIR, &t)
			if err != nil {
				fmt.Println("Error customizing disk image:", err)
				return err
			}
		}

		// Linux
		if !strings.Contains(operatingSystem, "win") {
			t := EmbedTool{filesystem: &scriptFS}

			err = CustomizeLinux(CustomizeDomainExec, disks, DIR, &t)
			if err != nil {
				fmt.Println("Error customizing disk image:", err)
				return err
			}
		}
	}

	return nil
}

func convertVirtV2vInPlace() error {
	args := []string{"-v", "-x", "-i", "libvirtxml"}
	args = append(args, "--root")
	if val, found := os.LookupEnv("V2V_RootDisk"); found {
		args = append(args, val)
	} else {
		args = append(args, "first")
	}
	args = append(args, "/mnt/v2v/input.xml")
	return executeVirtV2v("/usr/libexec/virt-v2v-in-place", args)
}

func virtV2vBuildCommand() (args []string, err error) {
	args = []string{"-v", "-x"}
	source := os.Getenv("V2V_source")

	requiredEnvVars := map[string][]string{
		VSPHERE: {"V2V_libvirtURL", "V2V_secretKey", "V2V_vmName"},
		OVA:     {"V2V_diskPath", "V2V_vmName"},
	}

	if envVars, ok := requiredEnvVars[source]; ok {
		if !checkEnvVariablesSet(envVars...) {
			return nil, fmt.Errorf("Following environment variables need to be defined: %v\n", envVars)
		}
	} else {
		providers := make([]string, len(requiredEnvVars))
		for key := range requiredEnvVars {
			providers = append(providers, key)
		}
		return nil, fmt.Errorf("virt-v2v supports the following providers: {%v}. Provided: %s\n", strings.Join(providers, ", "), source)
	}
	fmt.Println("Preparing virt-v2v")

	switch source {
	case VSPHERE:
		args = append(args, "--root")
		if checkEnvVariablesSet("V2V_RootDisk") {
			args = append(args, os.Getenv("V2V_RootDisk"))
		} else {
			args = append(args, "first")
		}
		args = append(args, "-i", "libvirt", "-ic", os.Getenv("V2V_libvirtURL"))
	case OVA:
		args = append(args, "-i", "ova", os.Getenv("V2V_diskPath"))
	}

	if err := os.MkdirAll(DIR, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Error creating directory: %v", err)
	}
	args = append(args, "-o", "local", "-os", DIR)

	//Disks on filesystem storage.
	if err = LinkDisks(FS, 15); err != nil {
		return
	}
	//Disks on block storage.
	if err = LinkDisks(BLOCK, 10); err != nil {
		return
	}

	if source == VSPHERE {
		args = append(args, "-ip", "/etc/secret/secretKey")

		if envStaticIPs := os.Getenv("V2V_staticIPs"); envStaticIPs != "" {
			for _, macToIp := range strings.Split(envStaticIPs, "_") {
				args = append(args, "--mac", macToIp)
			}
		}

		// Adds LUKS keys, if they exist
		luksArgs, err := addLUKSKeys()
		if err != nil {
			return nil, fmt.Errorf("Error adding LUKS keys: %v", err)
		}
		args = append(args, luksArgs...)

		if info, err := os.Stat(VDDK); err == nil && info.IsDir() {
			args = append(args,
				"-it", "vddk",
				"-io", fmt.Sprintf("vddk-libdir=%s", VDDK),
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
	}
	return args, nil
}

// addLUKSKeys checks the LUKS directory for key files and returns the appropriate
// arguments for a 'virt-' command to add these keys.
//
// Returns a slice of strings representing the LUKS key arguments, or an error if
// there's an issue accessing the directory or reading the files.
func addLUKSKeys() ([]string, error) {
	var luksArgs []string

	if _, err := os.Stat(LUKSDIR); err == nil {
		files, err := getFilesInPath(LUKSDIR)
		if err != nil {
			return nil, fmt.Errorf("Error reading files in LUKS directory: %v", err)
		}

		var luksFiles []string
		for _, file := range files {
			luksFiles = append(luksFiles, fmt.Sprintf("all:file:%s", file))
		}

		luksArgs = append(luksArgs, getScriptArgs("key", luksFiles...)...)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error accessing the LUKS directory: %v", err)
	}

	return luksArgs, nil
}

func convertVirtV2v() (err error) {
	source := os.Getenv("V2V_source")
	if source == VSPHERE {
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

	args, err := virtV2vBuildCommand()
	if err != nil {
		return
	}
	if err = executeVirtV2v("virt-v2v", args); err != nil {
		return
	}

	if xmlFilePath, err = getXMLFile(DIR, "xml"); err != nil {
		fmt.Println("Error getting XML file:", err)
		return err
	}

	err = customizeVM(source, xmlFilePath)
	if err != nil {
		fmt.Println("Error customizing the VM:", err)
		return err
	}

	http.HandleFunc("/ovf", ovfHandler)
	http.HandleFunc("/shutdown", shutdownHandler)
	server = &http.Server{Addr: ":8080"}

	fmt.Println("Starting server on :8080")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Error starting server: %v\n", err)
		return err
	}

	return
}

func getFilesInPath(rootPath string) (paths []string, err error) {
	files, err := os.ReadDir(rootPath)
	if err != nil {
		fmt.Println("Error reading the files in the directory ", err)
		return
	}
	for _, file := range files {
		if !file.IsDir() && !strings.HasPrefix(file.Name(), "..") {
			paths = append(paths, fmt.Sprintf("%s/%s", rootPath, file.Name()))
		}
	}
	return
}

func checkEnvVariablesSet(envVars ...string) bool {
	for _, v := range envVars {
		if os.Getenv(v) == "" {
			return false
		}
	}
	return true
}

func genName(diskNum int) string {
	if diskNum <= 0 {
		return ""
	}

	index := (diskNum - 1) % LETTERS_LENGTH
	cycles := (diskNum - 1) / LETTERS_LENGTH

	return genName(cycles) + string(LETTERS[index])
}

func LinkDisks(diskKind string, num int) (err error) {
	disks, err := filepath.Glob(diskKind)
	if err != nil {
		fmt.Println("Error getting disks ", err)
		return
	}

	for _, disk := range disks {
		diskNum, err := strconv.Atoi(disk[num:])
		if err != nil {
			fmt.Println("Error getting disks names ", err)
			return err
		}
		diskLink := fmt.Sprintf("%s/%s-sd%s", DIR, os.Getenv("V2V_vmName"), genName(diskNum+1))
		diskImgPath := disk
		if diskKind == FS {
			diskImgPath = fmt.Sprintf("%s/disk.img", disk)
		}
		if err = os.Symlink(diskImgPath, diskLink); err != nil {
			fmt.Println("Error creating disk link ", err)
			return err
		}
	}
	return
}

func executeVirtV2v(command string, args []string) error {
	v2vCmd := exec.Command(command, args...)
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

func getXMLFile(dir, fileExtension string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*."+fileExtension))
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return "", fmt.Errorf("XML file was not found")
}

func ovfHandler(w http.ResponseWriter, r *http.Request) {
	xmlData, err := ReadXMLFile(xmlFilePath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	_, err = w.Write(xmlData)
	if err == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		fmt.Printf("Error writing response: %v\n", err)
		http.Error(w, "Error writing response", http.StatusInternalServerError)
	}

}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Shutdown request received. Shutting down server.")
	w.WriteHeader(http.StatusNoContent)
	if err := server.Shutdown(context.Background()); err != nil {
		fmt.Printf("error shutting down server: %v\n", err)
	}
}
