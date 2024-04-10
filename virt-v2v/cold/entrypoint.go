package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

var UEFI_RE = regexp.MustCompile(`(?i)UEFI\s+bootloader?`)
var firmware = "bios"
var nameChanged bool

var (
	yamlFilePath string
	server       *http.Server
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
		fmt.Println("Warning customizing the VM failed:", err)
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
	virtV2vArgs = append(virtV2vArgs, "-o", "kubevirt")

	if checkEnvVariablesSet("V2V_NewName") {
		virtV2vArgs = append(virtV2vArgs, "-on", os.Getenv("V2V_NewName"))
		nameChanged = true
	}

	virtV2vArgs = append(virtV2vArgs, "-os", DIR)
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

		// Adds LUKS keys, if they exist
		luksArgs, err := addLUKSKeys()
		if err != nil {
			fmt.Println("Error adding LUKS kyes ", err)
			os.Exit(1)
		}
		virtV2vArgs = append(virtV2vArgs, luksArgs...)

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
			fmt.Println("Error reading files in LUKS directory", err)
			os.Exit(1)
		}

		var luksFiles []string
		for _, file := range files {
			luksFiles = append(luksFiles, fmt.Sprintf("all:file:%s", file))
		}

		luksArgs = append(luksArgs, getScriptArgs("key", luksFiles...)...)
	} else if !os.IsNotExist(err) {
		fmt.Println("Error accessing the LUKS directory", err)
		os.Exit(1)
	}

	return luksArgs, nil
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

	var diskSuffix string
	if nameChanged {
		diskSuffix = os.Getenv("V2V_newName")
	} else {
		diskSuffix = os.Getenv("V2V_vmName")
	}

	for _, disk := range disks {
		diskNum, err := strconv.Atoi(disk[num:])
		if err != nil {
			fmt.Println("Error getting disks names ", err)
			return err
		}
		diskLink := fmt.Sprintf("%s/%s-sd%s", DIR, diskSuffix, genName(diskNum+1))
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

func executeVirtV2v(args []string, source string) (err error) {
	virtV2vCmd := exec.Command(args[0], args[1:]...)
	virtV2vStdoutPipe, err := virtV2vCmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error setting up stdout pipe: %v\n", err)
		return
	}
	teeOut := io.TeeReader(virtV2vStdoutPipe, os.Stdout)

	var teeErr io.Reader
	if source == OVA {
		virtV2vStderrPipe, err := virtV2vCmd.StderrPipe()
		if err != nil {
			fmt.Printf("Error setting up stdout pipe: %v\n", err)
			return err
		}
		teeErr = io.TeeReader(virtV2vStderrPipe, os.Stderr)
	} else {
		virtV2vCmd.Stderr = os.Stderr
	}

	fmt.Println("exec ", virtV2vCmd)
	if err = virtV2vCmd.Start(); err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		return
	}

	virtV2vMonitorCmd := exec.Command("/usr/local/bin/virt-v2v-monitor")
	virtV2vMonitorCmd.Stdin = teeOut
	virtV2vMonitorCmd.Stdout = os.Stdout
	virtV2vMonitorCmd.Stderr = os.Stderr

	if err = virtV2vMonitorCmd.Start(); err != nil {
		fmt.Printf("Error executing monitor command: %v\n", err)
		return err
	}

	if source == OVA {
		scanner := bufio.NewScanner(teeErr)
		const maxCapacity = 1024 * 1024
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxCapacity)

		for scanner.Scan() {
			line := scanner.Bytes()
			if match := UEFI_RE.FindSubmatch(line); match != nil {
				fmt.Println("UEFI firmware detected")
				firmware = "efi"
			}
		}

		if err = scanner.Err(); err != nil {
			fmt.Println("Output query failed:", err)
			return err
		}
	}

	if err = virtV2vCmd.Wait(); err != nil {
		fmt.Printf("Error waiting for virt-v2v to finish: %v\n", err)
		return
	}

	// virt-v2v is done, we can close the pipe to virt-v2v-monitor
	writer.Close()

		fmt.Printf("Error waiting for virt-v2v-monitor to finish: %v\n", err)
		return err
	}

	return nil
}

func getYamlFile(dir, fileExtension string) (string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*."+fileExtension))
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return "", fmt.Errorf("yaml file was not found")
}

func vmHandler(w http.ResponseWriter, r *http.Request) {
	if yamlFilePath == "" {
		fmt.Println("Error: YAML file path is empty.")
		http.Error(w, "YAML file path is empty", http.StatusInternalServerError)
		return
	}

	yamlData, err := os.ReadFile(yamlFilePath)
	if err != nil {
		fmt.Printf("Error reading yaml file: %v\n", err)
		http.Error(w, "Error reading Yaml file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	_, err = w.Write(yamlData)
	if err != nil {
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
