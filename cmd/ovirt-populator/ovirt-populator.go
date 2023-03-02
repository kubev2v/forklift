package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"k8s.io/klog/v2"
)

type engineConfig struct {
	URL      string
	username string
	password string
	ca       string
}

type TransferProgress struct {
	Transferred uint64  `json:"transferred"`
	Description string  `json:"description"`
	Size        *uint64 `json:"size,omitempty"`
	Elapsed     float64 `json:"elapsed"`
}

func main() {
	var engineUrl, secretName, diskID, volPath, crName, crNamespace, namespace string
	// Populate args
	flag.StringVar(&engineUrl, "engine-url", "", "ovirt-engine url (https//engine.fqdn)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing oVirt credentials")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&volPath, "volume-path", "", "Volume path to populate")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")

	// Other args
	flag.StringVar(&namespace, "namespace", "konveyor-forklift", "Namespace to deploy controller")
	flag.Parse()

	populate(engineUrl, diskID, volPath)
}

func populate(engineURL, diskID, volPath string) {
	engineConfig := loadEngineConfig(engineURL)

	// Write credentials to files
	ovirtPass, err := os.Create("/tmp/ovirt.pass")
	if err != nil {
		klog.Fatalf("Failed to create ovirt.pass %v", err)
	}

	defer ovirtPass.Close()
	_, err = ovirtPass.Write([]byte(engineConfig.password))
	if err != nil {
		klog.Fatalf("Failed to write password to file: %v", err)
	}

	cert, err := os.Create("/tmp/ca.pem")
	if err != nil {
		klog.Fatalf("Failed to create ca.pem %v", err)
	}

	defer cert.Close()
	_, err = cert.Write([]byte(engineConfig.ca))
	if err != nil {
		klog.Fatalf("Failed to write CA to file: %v", err)
	}

	args := []string{
		"download-disk",
		"--output", "json",
		"--engine-url=" + engineConfig.URL,
		"--username=" + engineConfig.username,
		"--password-file=/tmp/ovirt.pass",
		"--cafile=" + "/tmp/ca.pem",
		"-f", "raw",
		diskID,
		volPath,
	}
	cmd := exec.Command("ovirt-img", args...)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)
	klog.Info(fmt.Sprintf("Running command: %s", cmd.String()))

	go func() {
		for scanner.Scan() {
			progressOutput := TransferProgress{}
			text := scanner.Text()
			klog.Info(text)
			err = json.Unmarshal([]byte(text), &progressOutput)
			if err != nil {
				klog.Error(err)
			}

			//TODO add progress mechanism to the disk transfer
			/*if progressOutput.Size != nil {
				// We have to get it in the loop to avoid a conflict error
				populatorCr, err := client.Resource(gvr).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
				if err != nil {
					klog.Error(err.Error())
				}

				status := map[string]interface{}{"progress": fmt.Sprintf("%d", progressOutput.Transferred)}
				unstructured.SetNestedField(populatorCr.Object, status, "status")

				_, err = client.Resource(gvr).Namespace(crNamespace).Update(context.TODO(), populatorCr, metav1.UpdateOptions{})

				if err != nil {
					klog.Error(err)
				}
			}*/
		}

		done <- struct{}{}
	}()

	err = cmd.Start()
	if err != nil {
		klog.Fatal(err)
	}

	<-done
	err = cmd.Wait()
	if err != nil {
		klog.Fatal(err)
	}
}

func loadEngineConfig(engineURL string) engineConfig {
	user, err := os.ReadFile("/etc/secret-volume/user")
	if err != nil {
		klog.Fatal(err.Error())
	}
	pass, err := os.ReadFile("/etc/secret-volume/password")
	if err != nil {
		klog.Fatal(err.Error())
	}
	ca, err := os.ReadFile("/etc/secret-volume/cacert")
	if err != nil {
		klog.Fatal(err.Error())
	}

	return engineConfig{
		URL:      engineURL,
		username: string(user),
		password: string(pass),
		ca:       string(ca),
	}
}
