package main

import (
	"flag"
	"io"
	"os"
	"strings"
	"time"

	libclient "github.com/kubev2v/forklift/pkg/lib/client/openstack"
	"github.com/kubev2v/forklift/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/klog/v2"
)

type AppConfig struct {
	identityEndpoint string
	imageID          string
	crNamespace      string
	crName           string
	secretName       string
	ownerUID         string
	pvcSize          int64
	volumePath       string
}

func main() {
	config := &AppConfig{}
	flag.StringVar(&config.identityEndpoint, "endpoint", "", "endpoint URL (https://openstack.example.com:5000/v2.0)")
	flag.StringVar(&config.secretName, "secret-name", "", "secret containing OpenStack credentials")
	flag.StringVar(&config.imageID, "image-id", "", "Openstack image ID")
	flag.StringVar(&config.volumePath, "volume-path", "", "Path to populate")
	flag.StringVar(&config.crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&config.crNamespace, "cr-namespace", "", "Custom Resource instance namespace")
	flag.StringVar(&config.ownerUID, "owner-uid", "", "Owner UID (usually PVC UID)")
	flag.Int64Var(&config.pvcSize, "pvc-size", 0, "Size of pvc (in bytes)")
	flag.Parse()

	if config.pvcSize <= 0 {
		klog.Fatal("pvc-size must be greater than 0")
	}

	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatal(err)
	}

	metrics.StartPrometheusEndpoint(certsDirectory)

	populate(config)
}

func populate(config *AppConfig) {
	client := createClient(config)
	downloadAndSaveImage(client, config)
}

func createClient(config *AppConfig) *libclient.Client {
	options := readOptions()
	client := &libclient.Client{
		URL:     config.identityEndpoint,
		Options: options,
	}

	err := client.Connect()
	if err != nil {
		klog.Fatal(err)
	}

	return client
}

func downloadAndSaveImage(client *libclient.Client, config *AppConfig) {
	klog.Info("Downloading the image: ", config.imageID)
	imageReader, err := client.DownloadImage(config.imageID)
	if err != nil {
		klog.Fatal(err)
	}

	defer imageReader.Close()

	file := openFile(config.volumePath)
	defer file.Close()

	progressVec := createProgressCounter()
	writeData(imageReader, file, config, progressVec)
}

func createProgressCounter() *prometheus.CounterVec {
	progressVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "openstack_populator_progress",
			Help: "Progress of volume population",
		},
		[]string{"ownerUID"},
	)

	if err := prometheus.Register(progressVec); err != nil {
		klog.Error("Prometheus progress counter not registered:", err)
	}

	return progressVec
}

func openFile(volumePath string) *os.File {
	flags := os.O_RDWR
	if strings.HasSuffix(volumePath, "disk.img") {
		flags |= os.O_CREATE
	}
	file, err := os.OpenFile(volumePath, flags, 0650)
	if err != nil {
		klog.Fatal(err)
	}
	return file
}

func writeData(reader io.ReadCloser, file *os.File, config *AppConfig, progress *prometheus.CounterVec) {
	countingReader := &CountingReader{reader: reader, total: config.pvcSize, read: new(int64)}
	done := make(chan bool)

	go reportProgress(done, countingReader, progress, config)

	if _, err := io.Copy(file, countingReader); err != nil {
		klog.Fatal(err)
	}
	done <- true
}

func reportProgress(done chan bool, countingReader *CountingReader, progress *prometheus.CounterVec, config *AppConfig) {
	for {
		select {
		case <-done:
			finalizeProgress(progress, config.ownerUID)
			return
		default:
			updateProgress(countingReader, progress, config.ownerUID)
			time.Sleep(1 * time.Second)
		}
	}
}

func finalizeProgress(progress *prometheus.CounterVec, ownerUID string) {
	currentVal := progress.WithLabelValues(ownerUID)

	var metric dto.Metric
	if err := currentVal.Write(&metric); err != nil {
		klog.Error("Error reading current progress:", err)
		return
	}

	if metric.Counter != nil {
		remainingProgress := 100 - *metric.Counter.Value
		if remainingProgress > 0 {
			currentVal.Add(remainingProgress)
		}
	}

	klog.Info("Finished populating the volume. Progress: 100%")
}

func updateProgress(countingReader *CountingReader, progress *prometheus.CounterVec, ownerUID string) {
	if countingReader.total <= 0 {
		return
	}

	metric := &dto.Metric{}
	if err := progress.WithLabelValues(ownerUID).Write(metric); err != nil {
		klog.Errorf("updateProgress: failed to write metric; %v", err)
	}

	currentProgress := (float64(*countingReader.read) / float64(countingReader.total)) * 100

	if currentProgress > *metric.Counter.Value {
		progress.WithLabelValues(ownerUID).Add(currentProgress - *metric.Counter.Value)
	}

	klog.Info("Progress: ", int64(currentProgress), "%")
}

func readOptions() map[string]string {
	options := map[string]string{}

	// List of options to read from environment variables
	envOptions := []string{
		"regionName", "authType", "username", "userID", "password",
		"applicationCredentialID", "applicationCredentialName", "applicationCredentialSecret",
		"token", "systemScope", "projectName", "projectID", "userDomainName",
		"userDomainID", "projectDomainName", "projectDomainID", "domainName",
		"domainID", "defaultDomain", "insecureSkipVerify", "cacert", "availability",
	}

	klog.Info("Options:")
	for _, option := range envOptions {
		value := os.Getenv(option)
		options[option] = value
		if sensitiveInfo(option) {
			value = strings.Repeat("*", len(value))
		}
		klog.Info(" - ", option, " = ", value)
	}
	return options
}

func sensitiveInfo(option string) bool {
	return option == "password" || option == "applicationCredentialSecret" || option == "token"
}

type CountingReader struct {
	reader io.ReadCloser
	read   *int64
	total  int64
}

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	*cr.read += int64(n)
	return n, err
}
