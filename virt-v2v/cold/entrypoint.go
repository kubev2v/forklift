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
	vSphere = "vSphere"
	DIR     = "/var/tmp/v2v"
	FS      = "/mnt/disks/disk[0-9]*"
	Block   = "/dev/block[0-9]*"
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
	source := os.Getenv("V2V_source")
	if source == vSphere {
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

	if err := executeVirtV2v(buildCommand()); err != nil {
		fmt.Println("Error executing virt-v2v command ", err)
		os.Exit(1)
	}

	var err error
	xmlFilePath, err = getXMLFile(DIR, "xml")
	if err != nil {
		fmt.Println("Error getting XML file:", err)
		os.Exit(1)
	}

	err = customizeVM(source, xmlFilePath)
	if err != nil {
		fmt.Println("Error customizing the VM:", err)
		os.Exit(1)
	}

	http.HandleFunc("/ovf", ovfHandler)
	http.HandleFunc("/shutdown", shutdownHandler)
	server = &http.Server{Addr: ":8080"}

	fmt.Println("Starting server on :8080")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Error starting server: %v\n", err)
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
	if source == vSphere {
		// Windows
		if strings.Contains(operatingSystem, "win") {
			err = CustomizeWindows(disks)
			if err != nil {
				fmt.Println("Error customizing disk image:", err)
				return err
			}
		}
	}

	return nil
}

func buildCommand() []string {
	virtV2vArgs := []string{"-v", "-x"}
	source := os.Getenv("V2V_source")

	if !isValidSource(source) {
		fmt.Printf("virt-v2v supports the following providers: {OVA, vSphere}. Provided: %s\n", source)
		os.Exit(1)
	}

	requiredEnvVars := map[string][]string{
		vSphere: {"V2V_libvirtURL", "V2V_secretKey", "V2V_vmName"},
		OVA:     {"V2V_diskPath", "V2V_vmName"},
	}

	if envVars, ok := requiredEnvVars[source]; ok {
		if !checkEnvVariablesSet(envVars...) {
			fmt.Printf("Following environment variables need to be defined: %v\n", envVars)
			os.Exit(1)
		}
	}

	fmt.Println("Preparing virt-v2v")

	switch source {
	case vSphere:
		virtV2vArgs = append(virtV2vArgs, "--root")
		if checkEnvVariablesSet("V2V_RootDisk") {
			virtV2vArgs = append(virtV2vArgs, os.Getenv("V2V_RootDisk"))
		} else {
			virtV2vArgs = append(virtV2vArgs, "first")
		}
		virtV2vArgs = append(virtV2vArgs, "-i", "libvirt", "-ic", os.Getenv("V2V_libvirtURL"))
	case OVA:
		virtV2vArgs = append(virtV2vArgs, "-i", "ova", os.Getenv("V2V_diskPath"))
	}

	if err := os.MkdirAll(DIR, os.ModePerm); err != nil {
		fmt.Println("Error creating directory  ", err)
		os.Exit(1)
	}
	virtV2vArgs = append(virtV2vArgs, "-o", "local", "-os", DIR)

	//Disks on filesystem storage.
	if err := LinkDisks(FS, 15); err != nil {
		os.Exit(1)
	}
	//Disks on block storage.
	if err := LinkDisks(Block, 10); err != nil {
		os.Exit(1)
	}

	if source == vSphere {
		virtV2vArgs = append(virtV2vArgs, "-ip", "/etc/secret/secretKey")

		if envStaticIPs := os.Getenv("V2V_staticIPs"); envStaticIPs != "" {
			for _, macToIp := range strings.Split(envStaticIPs, "_") {
				virtV2vArgs = append(virtV2vArgs, "--mac", macToIp)
			}
		}
		// Adds LUKS keys, if exist.
		if _, err := os.Stat(LUKSDIR); err == nil {
			files, err := getFilesInPath(LUKSDIR)
			if err != nil {
				fmt.Println("Error reading files in LUKS directory ", err)
				os.Exit(1)
			}
			for _, file := range files {
				virtV2vArgs = append(virtV2vArgs, "--key", fmt.Sprintf("all:file:%s", file))
			}
		} else if !os.IsNotExist(err) {
			fmt.Println("Error accessing the LUKS directory ", err)
			os.Exit(1)
		}

		if info, err := os.Stat(VDDK); err == nil && info.IsDir() {
			virtV2vArgs = append(virtV2vArgs,
				"-it", "vddk",
				"-io", fmt.Sprintf("vddk-libdir=%s", VDDK),
				"-io", fmt.Sprintf("vddk-thumbprint=%s", os.Getenv("V2V_fingerprint")),
			)
		}
		var extraArgs []string
		if envExtraArgs := os.Getenv("V2V_extra_args"); envExtraArgs != "" {
			if err := json.Unmarshal([]byte(envExtraArgs), &extraArgs); err != nil {
				fmt.Println("Error parsing extra arguments ", err)
				os.Exit(1)
			}
		}
		virtV2vArgs = append(virtV2vArgs, extraArgs...)

		virtV2vArgs = append(virtV2vArgs, "--", os.Getenv("V2V_vmName"))
	}
	return virtV2vArgs
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

func executeVirtV2v(args []string) error {
	v2vCmd := exec.Command("virt-v2v", args...)
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

func isValidSource(source string) bool {
	switch source {
	case OVA, vSphere:
		return true
	default:
		return false
	}
}
