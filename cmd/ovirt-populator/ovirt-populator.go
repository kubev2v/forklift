package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

type engineConfig struct {
	URL      string
	username string
	password string
	cacert   string
	insecure bool
}

type TransferProgress struct {
	Transferred uint64  `json:"transferred"`
	Description string  `json:"description"`
	Size        *uint64 `json:"size,omitempty"`
	Elapsed     float64 `json:"elapsed"`
}

func main() {
	var engineUrl, secretName, diskID, volPath, crName, crNamespace, namespace, ownerUID string
	var pvcSize *int64

	// Populate args
	flag.StringVar(&engineUrl, "engine-url", "", "ovirt-engine url (https//engine.fqdn)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing oVirt credentials")
	flag.StringVar(&diskID, "disk-id", "", "ovirt-engine disk id")
	flag.StringVar(&volPath, "volume-path", "", "Volume path to populate")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&ownerUID, "owner-uid", "", "Owner UID (usually PVC UID)")
	pvcSize = flag.Int64("pvc-size", 0, "Size of pvc (in bytes)")

	// Other args
	flag.StringVar(&namespace, "namespace", "konveyor-forklift", "Namespace to deploy controller")
	flag.Parse()

	populate(engineUrl, diskID, volPath, ownerUID, *pvcSize)
}

func populate(engineURL, diskID, volPath, ownerUID string, pvcSize int64) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)
	progressCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ovirt_progress",
			Help: "Progress of volume population",
		},
		[]string{"ownerUID"},
	)
	if err := prometheus.Register(progressCounter); err != nil {
		klog.Error("Prometheus progress gauge not registered:", err)
	} else {
		klog.Info("Prometheus progress gauge registered.")
	}

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

	args := createCommandArguments(&engineConfig, diskID, volPath)
	cmd := exec.Command("ovirt-img", args...)
	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan struct{})
	scanner := bufio.NewScanner(r)
	klog.Info(fmt.Sprintf("Running command: %s", cmd.String()))

	go func() {
		var currentProgress float64
		total := pvcSize
		metric := &dto.Metric{}

		for scanner.Scan() {
			progressOutput := TransferProgress{}
			text := scanner.Text()
			klog.Info(text)
			err = json.Unmarshal([]byte(text), &progressOutput)
			if err != nil {
				var syntaxError *json.SyntaxError
				if !errors.As(err, &syntaxError) {
					klog.Error(err)
				}
			}
			if total > 0 {
				currentProgress = (float64(progressOutput.Transferred) / float64(total)) * 100
				err = progressCounter.WithLabelValues(ownerUID).Write(metric)
				if err != nil {
					klog.Error(err)
				} else if currentProgress > metric.Counter.GetValue() {
					progressCounter.WithLabelValues(ownerUID).Add(currentProgress - metric.Counter.GetValue())
				}
			}
		}

		metric.GetCounter().Value = ptr.To[float64](100)
		err = progressCounter.WithLabelValues(ownerUID).Write(metric)

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

func createCommandArguments(config *engineConfig, diskID, volPath string) []string {
	if config.insecure {
		return []string{
			"download-disk",
			"--output", "json",
			"--engine-url=" + config.URL,
			"--username=" + config.username,
			"--password-file=/tmp/ovirt.pass",
			"--insecure",
			"-f", "raw",
			diskID,
			volPath,
		}
	} else {
		// for secure connection use the ca cert
		cert, err := os.Create("/tmp/ca.pem")
		if err != nil {
			klog.Fatalf("Failed to create ca.pem %v", err)
		}

		defer cert.Close()
		_, err = cert.Write([]byte(config.cacert))
		if err != nil {
			klog.Fatalf("Failed to write CA to file: %v", err)
		}

		return []string{
			"download-disk",
			"--output", "json",
			"--engine-url=" + config.URL,
			"--username=" + config.username,
			"--password-file=/tmp/ovirt.pass",
			"--cafile=" + "/tmp/ca.pem",
			"-f", "raw",
			diskID,
			volPath,
		}
	}
}

func loadEngineConfig(engineURL string) engineConfig {
	user := os.Getenv("user")
	pass := os.Getenv("password")

	insecureSkipVerify, found := os.LookupEnv("insecureSkipVerify")
	if !found {
		insecureSkipVerify = "false"
	}

	insecure, err := strconv.ParseBool(string(insecureSkipVerify))
	if err != nil {
		klog.Fatal(err.Error())
	}

	//If the insecure option is set, the ca file field in the secret is not required.
	cacert := os.Getenv("cacert")

	return engineConfig{
		URL:      engineURL,
		username: user,
		password: pass,
		cacert:   cacert,
		insecure: insecure,
	}
}
