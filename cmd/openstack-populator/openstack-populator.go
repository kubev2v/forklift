package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	libclient "github.com/konveyor/forklift-controller/pkg/lib/client/openstack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

func main() {
	var (
		identityEndpoint string
		imageID          string
		crNamespace      string
		crName           string
		secretName       string

		volumePath string
	)

	klog.InitFlags(nil)

	// Main arg
	flag.StringVar(&identityEndpoint, "endpoint", "", "endpoint URL (https://openstack.example.com:5000/v2.0)")
	flag.StringVar(&secretName, "secret-name", "", "secret containing OpenStack credentials")

	flag.StringVar(&imageID, "image-id", "", "Openstack image ID")
	flag.StringVar(&volumePath, "volume-path", "", "Path to populate")
	flag.StringVar(&crName, "cr-name", "", "Custom Resource instance name")
	flag.StringVar(&crNamespace, "cr-namespace", "", "Custom Resource instance namespace")

	flag.Parse()

	populate(volumePath, identityEndpoint, secretName, imageID)
}

func populate(fileName, identityEndpoint, secretName, imageID string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)
	progressGague := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "volume_populators",
			Name:      "openstack_volume_populator",
			Help:      "Amount of data transferred",
		},
		[]string{"image_id"},
	)

	if err := prometheus.Register(progressGague); err != nil {
		klog.Error("Prometheus progress counter not registered:", err)
	} else {
		klog.Info("Prometheus progress counter registered.")
	}

	options := readOptions()

	client := &libclient.Client{
		URL:     identityEndpoint,
		Options: options,
	}

	err := client.Connect()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("Downloading the image: ", imageID)
	imageReader, err := client.DownloadImage(imageID)
	if err != nil {
		klog.Fatal(err)
	}
	defer imageReader.Close()

	if err != nil {
		klog.Fatal(err)
	}
	flags := os.O_RDWR
	if strings.HasSuffix(fileName, "disk.img") {
		flags |= os.O_CREATE
	}

	klog.Info("Saving the image to: ", fileName)
	file, err := os.OpenFile(fileName, flags, 0650)
	if err != nil {
		klog.Fatal(err)
	}
	defer file.Close()

	err = writeData(imageReader, file, imageID, progressGague)
	if err != nil {
		klog.Fatal(err)
	}
}

type CountingReader struct {
	reader io.ReadCloser
	total  *int64
}

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	*cr.total += int64(n)
	return n, err
}

func writeData(reader io.ReadCloser, file *os.File, imageID string, progress *prometheus.GaugeVec) error {
	total := new(int64)
	countingReader := CountingReader{reader, total}

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				klog.Info("Total: ", *total)
				klog.Info("Finished!")
				return
			default:
				progress.WithLabelValues(imageID).Set(float64(*total))
				klog.Info("Transferred: ", *total)
				time.Sleep(3 * time.Second)
			}
		}
	}()

	if _, err := io.Copy(file, &countingReader); err != nil {
		klog.Fatal(err)
	}
	done <- true
	progress.WithLabelValues(imageID).Set(float64(*total))

	return nil
}

func readOptions() (options map[string]string) {
	options = map[string]string{}

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

		// Mask sensitive information
		if option == "password" || option == "applicationCredentialSecret" || option == "token" {
			value = strings.Repeat("*", len(value))
		}

		klog.Info(" - ", option, " = ", value)
	}

	return
}
