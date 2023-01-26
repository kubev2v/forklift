package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	populator_machinery "github.com/kubev2v/lib-volume-populator/populator-machinery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	prefix     = "forklift.konveyor.io"
	mountPath  = "/mnt/"
	devicePath = "/dev/block"
)

const (
	groupName  = "forklift.konveyor.io"
	apiVersion = "v1beta1"
	kind       = "OvirtVolumePopulator"
	resource   = "ovirtvolumepopulators"
)

func main() {
	var mode, engineUrl, secretName, diskID, volPath, crName, crNamespace, httpEndpoint, metricsPath, masterURL, kubeconfig, imageName, namespace string

	// Main arg
	flag.StringVar(&mode, "mode", "", "Mode to run in (controller, populate)")
	// Populate args
	flag.StringVar(&engineUrl, "engine-url", "", "ovirt-engine url (https//engine.fqdn)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing oVirt credentials")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&volPath, "volume-path", "", "Volume path to populate")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")

	// Controller args
	if f := flag.Lookup("kubeconfig"); f != nil {
		kubeconfig = f.Value.String()
	} else {
		flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	}
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&imageName, "image-name", "", "Image to use for populating")
	// Metrics args
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	// Other args
	flag.StringVar(&namespace, "namespace", "konveyor-forklift", "Namespace to deploy controller")
	flag.Parse()

	switch mode {
	case "controller":
		var (
			gk  = schema.GroupKind{Group: groupName, Kind: kind}
			gvr = schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}
		)
		populator_machinery.RunController(masterURL, kubeconfig, imageName, httpEndpoint, metricsPath,
			namespace, prefix, gk, gvr, mountPath, devicePath, getPopulatorPodArgs)
	case "populate":
		populate(masterURL, kubeconfig, crName, engineUrl, secretName, diskID, volPath, namespace, crNamespace)
	default:
		klog.Fatalf("Invalid mode: %s", mode)
	}
}

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

func getPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var ovirtVolumePopulator v1beta1.OvirtVolumePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ovirtVolumePopulator)
	args := []string{"--mode=populate"}
	if nil != err {
		return nil, err
	}

	if rawBlock {
		args = append(args, "--file-name="+devicePath)
	} else {
		args = append(args, "--file-name="+mountPath+"disk.img")
	}

	args = append(args, "--secret-name="+ovirtVolumePopulator.Spec.EngineSecretName)
	args = append(args, "--disk-id="+ovirtVolumePopulator.Spec.DiskID)
	args = append(args, "--engine-url="+ovirtVolumePopulator.Spec.EngineURL)
	args = append(args, "--cr-name="+ovirtVolumePopulator.Name)
	args = append(args, "--cr-namespace="+ovirtVolumePopulator.Namespace)

	return args, nil
}

func populate(masterURL, kubeconfig, crName, engineURL, secretName, diskID, volPath, namespace, crNamespace string) {
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
