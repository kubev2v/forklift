package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/konveyor/forklift-controller/ovirt-imageio/pkg/v1beta1"
	"os"
	"os/exec"

	populator_machinery "github.com/kubernetes-csi/lib-volume-populator/populator-machinery"
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

var version = "unknown"

const (
	groupName  = "forklift.konveyor.io"
	apiVersion = "v1beta1"
	kind       = "OvirtImageIOPopulator"
	resource   = "ovirtimageiopopulators"
)

func main() {
	var (
		mode         string
		engineUrl    string
		secretName   string
		diskID       string
		fileName     string
		crName       string
		crNamespace  string
		httpEndpoint string
		metricsPath  string
		masterURL    string
		kubeconfig   string
		imageName    string
		showVersion  bool
		namespace    string
	)

	// Main arg
	flag.StringVar(&mode, "mode", "", "Mode to run in (controller, populate)")
	// Populate args
	flag.StringVar(&engineUrl, "engine-url", "", "ovirt-engine url (https//engine.fqdn)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing oVirt credentials")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&fileName, "file-name", "", "File name to populate")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instancce namespace")

	// Controller args
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&imageName, "image-name", "", "Image to use for populating")
	// Metrics args
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")
	// Other args
	flag.BoolVar(&showVersion, "version", false, "display the version string")
	flag.StringVar(&namespace, "namespace", "konveyor-forklift", "Namespace to deploy controller")
	flag.Parse()

	if showVersion {
		fmt.Println(os.Args[0], version)
		os.Exit(0)
	}

	switch mode {
	case "controller":
		var (
			gk  = schema.GroupKind{Group: groupName, Kind: kind}
			gvr = schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}
		)
		populator_machinery.RunController(masterURL, kubeconfig, imageName, httpEndpoint, metricsPath,
			namespace, prefix, gk, gvr, mountPath, devicePath, getPopulatorPodArgs)
	case "populate":
		populate(masterURL, kubeconfig, crName, engineUrl, secretName, diskID, fileName, namespace, crNamespace)
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

func getSecret(secretName, engineURL, namespace string) engineConfig {
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
	var ovirtImageIOPopulator v1beta1.OvirtImageIOPopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ovirtImageIOPopulator)
	args := []string{"--mode=populate"}
	if nil != err {
		return nil, err
	}

	if rawBlock {
		args = append(args, "--file-name="+devicePath)
	} else {
		args = append(args, "--file-name="+mountPath+"disk.img")
	}

	args = append(args, "--secret-name="+ovirtImageIOPopulator.Spec.EngineSecretName)
	args = append(args, "--disk-id="+ovirtImageIOPopulator.Spec.DiskID)
	args = append(args, "--engine-url="+ovirtImageIOPopulator.Spec.EngineURL)
	args = append(args, "--cr-name="+ovirtImageIOPopulator.Name)
	args = append(args, "--cr-namespace="+ovirtImageIOPopulator.Namespace)

	return args, nil
}

func populate(masterURL, kubeconfig, crName, engineURL, secretName, diskID, fileName, namespace, crNamespace string) {
	engineConfig := getSecret(secretName, engineURL, namespace)

	// Write credentials to files
	ovirtPass, err := os.Create("/tmp/ovirt.pass")
	if err != nil {
		klog.Fatalf("Failed to create ovirt.pass %s", err)
	}

	defer ovirtPass.Close()
	if err != nil {
		klog.Fatalf("Failed to create file %s", err)
	}
	ovirtPass.Write([]byte(engineConfig.password))

	cert, err := os.Create("/tmp/ca.pem")
	if err != nil {
		klog.Fatalf("Failed to create ca.pem %s", err)
	}

	defer cert.Close()
	if err != nil {
		klog.Fatalf("Failed to create file %s", err)
	}

	cert.Write([]byte(engineConfig.ca))

	args := []string{
		"download-disk",
		"--log-level", "debug",
		"--output", "json",
		"--engine-url=" + engineConfig.URL,
		"--username=" + engineConfig.username,
		"--password-file=/tmp/ovirt.pass",
		"--cafile=" + "/tmp/ca.pem",
		"-f", "raw",
		diskID,
		fileName,
	}

	if err != nil {
		klog.Fatal(err.Error())
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
