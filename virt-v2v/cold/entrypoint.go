package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	OVA     = "ova"
	vSphere = "vSphere"
	DIR     = "/var/tmp/v2v"
	FS      = "/mnt/disks/disk[0-9]*"
	Block   = "/dev/block[0-9]*"
	VDDK    = "/opt/vmware-vix-disklib-distrib"
)

type Domain struct {
	XMLName xml.Name `xml:"domain"`
	Name    string   `xml:"name"`
	OS      OS       `xml:"os"`
}

type OS struct {
	Type   OSType `xml:"type"`
	Loader Loader `xml:"loader"`
	Nvram  Nvram  `xml:"nvram"`
}

type OSType struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Content string `xml:",chardata"`
}

type Loader struct {
	Readonly string `xml:"readonly,attr"`
	Type     string `xml:"type,attr"`
	Secure   string `xml:"secure,attr"`
	Path     string `xml:",chardata"`
}

type Nvram struct {
	Template string `xml:"template,attr"`
}

var (
	xmlFilePath      string
	requestProcessed = make(chan bool)
)

func main() {
	virtV2vArgs := []string{"virt-v2v", "-v", "-x"}
	source := os.Getenv("V2V_source")
	//source := OVA

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
		virtV2vArgs = append(virtV2vArgs, "-ip", "/etc/secret/secretKey")

		if info, err := os.Stat(VDDK); err == nil && info.IsDir() {
			virtV2vArgs = append(virtV2vArgs,
				"-it", "vddk",
				"-io", fmt.Sprintf("vddk-libdir=%s", VDDK),
				"-io", fmt.Sprintf("vddk-thumbprint=%s", os.Getenv("V2V_fingerprint")),
			)
		}
		virtV2vArgs = append(virtV2vArgs, "--", os.Getenv("V2V_vmName"))
	}

	if err := executeVirtV2v(virtV2vArgs); err != nil {
		fmt.Println("Error executing virt-v2v command ", err)
		os.Exit(1)
	}

	if source == OVA {
		var err error
		xmlFilePath, err = waitForXMLFile(DIR, "xml", 5, 60)
		if err != nil {
			fmt.Println("Error waiting for XML file:", err)
			return
		}

		http.HandleFunc("/firmware", firmwareHandler)
		server := &http.Server{Addr: ":8080"}

		go func() {
			// Wait for the first request to be processed
			<-requestProcessed
			fmt.Println("Shutting down server.")
			if err := server.Shutdown(context.Background()); err != nil {
				fmt.Printf("Error shutting down server: %v\n", err)
			}
		}()

		fmt.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
		}

		fmt.Println("Server stopped")
	}
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

	letters := []int32("abcdefghijklmnopqrstuvwxyz")
	index := (diskNum - 1) % len(letters)
	cycels := (diskNum - 1) % len(letters)

	return genName(cycels) + string(letters[index])
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
			fmt.Println("Error geting disks names ", err)
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

func executeVirtV2v(args []string) (err error) {
	virtV2vCmd := exec.Command("bash", "-c", strings.Join(args, " "), "|& /usr/local/bin/virt-v2v-monitor")
	virtV2vCmd.Stdout = os.Stdout
	virtV2vCmd.Stderr = os.Stderr
	if err = virtV2vCmd.Run(); err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		return
	}
	return
}

func waitForXMLFile(dir, fileExtension string, timeoutMinutes, checkIntervalSeconds int) (string, error) {
	deadline := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	for time.Now().Before(deadline) {
		files, err := filepath.Glob(filepath.Join(dir, "*."+fileExtension))
		if err != nil {
			return "", err
		}
		if len(files) > 0 {
			return files[0], nil
		}
		time.Sleep(time.Duration(checkIntervalSeconds) * time.Second)
	}
	return "", fmt.Errorf("timeout reached without finding an XML file")
}

func firmwareHandler(w http.ResponseWriter, r *http.Request) {

	if xmlFilePath == "" {
		fmt.Println("Error XML file path is empty.")
		return
	}

	firmware, err := getFirmwareFromConfig(xmlFilePath)
	if err != nil {
		fmt.Println("Error getting firmware from XML file:", err)
		return
	}

	jsonData, err := json.Marshal(map[string]string{"firmware": firmware})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error marshaling JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)

	requestProcessed <- true
}

func readConfFromXML(xmlFilePath string) (*Domain, error) {
	var domain Domain

	xmlFile, err := os.Open(xmlFilePath)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)

	err = decoder.Decode(&domain)
	if err != nil {
		return &domain, err
	}
	return &domain, nil
}

func getFirmwareFromConfig(xmlFilePath string) (conf string, err error) {
	xmlConf, err := readConfFromXML(xmlFilePath)
	if err != nil {
		return
	}

	path := xmlConf.OS.Loader.Path
	if strings.Contains(path, "OVMF") {
		return "uefi", nil
	}
	return "bios", nil
}
