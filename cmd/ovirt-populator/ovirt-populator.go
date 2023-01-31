package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

const (
	groupName  = "forklift.konveyor.io"
	apiVersion = "v1beta1"
	resource   = "ovirtvolumepopulators"
)

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

	populate(crName, engineUrl, secretName, diskID, volPath, namespace, crNamespace)
}

func populate(crName, engineURL, secretName, diskID, volPath, namespace, crNamespace string) {
	engineConfig := loadEngineConfig(secretName, engineURL, namespace)

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
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err.Error())
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err.Error())
	}

	gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}
	cmd := exec.Command("ovirt-img", args...)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)

	go func() {
		for scanner.Scan() {
			progressOutput := TransferProgress{}
			text := scanner.Text()
			klog.Info(text)
			err = json.Unmarshal([]byte(text), &progressOutput)
			if err != nil {
				klog.Error(err)
			}

			if progressOutput.Size != nil {
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
			}
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
		klog.Error(err)
	}
}

func loadEngineConfig(secretName, engineURL, namespace string) engineConfig {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err.Error())
	}

	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		klog.Fatal(err.Error())
	}

	return engineConfig{
		URL:      engineURL,
		username: string(secret.Data["user"]),
		password: string(secret.Data["password"]),
		ca:       string(secret.Data["cacert"]),
	}
}
