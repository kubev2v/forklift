package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/utils/openstack/clientconfig"
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

type openstackConfig struct {
	username           string
	password           string
	domainName         string
	projectName        string
	insecureSkipVerify string
	region             string
	cacert             string
}

func loadConfig(secretName, endpoint string) openstackConfig {
	username, err := os.ReadFile("/etc/secret-volume/username")
	if err != nil {
		klog.Fatal(err.Error())
	}
	password, err := os.ReadFile("/etc/secret-volume/password")
	if err != nil {
		klog.Fatal(err.Error())
	}
	projectName, err := os.ReadFile("/etc/secret-volume/projectName")
	if err != nil {
		klog.Fatal(err.Error())
	}
	region, err := os.ReadFile("/etc/secret-volume/regionName")
	if err != nil {
		klog.Fatal(err.Error())
	}
	domainName, err := os.ReadFile("/etc/secret-volume/domainName")
	if err != nil {
		klog.Fatal(err.Error())
	}
	insecureSkipVerify, err := os.ReadFile("/etc/secret-volume/insecureSkipVerify")
	if err != nil {
		klog.Error(err.Error())
		insecureSkipVerify = []byte("false")
	}
	cacert, err := os.ReadFile("/etc/secret-volume/cacert")
	if err != nil {
		klog.Error(err.Error())
	}

	return openstackConfig{
		username:           string(username),
		password:           string(password),
		insecureSkipVerify: string(insecureSkipVerify),
		projectName:        string(projectName),
		region:             string(region),
		domainName:         string(domainName),
		cacert:             string(cacert),
	}
}

func populate(fileName, endpoint, secretName, imageID string) {
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

	config := loadConfig(secretName, endpoint)
	var tlsConfig *tls.Config
	var err error

	if config.insecureSkipVerify == "true" {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	} else if config.cacert != "" {
		cacert := []byte(config.cacert)
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			klog.Fatal("Failed to parse certificate")
		}
		tlsConfig = &tls.Config{RootCAs: roots}
	}

	clientOpts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     endpoint,
			DomainName:  config.domainName,
			Username:    config.username,
			Password:    config.password,
			ProjectName: config.projectName,
			AllowReauth: true,
		},
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}

	provider, err := clientconfig.AuthenticatedClient(clientOpts)
	if err != nil {
		klog.Fatal(err)
	}

	imageService, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{Region: config.region})
	if err != nil {
		klog.Fatal(err)
	}

	image, err := imagedata.Download(imageService, imageID).Extract()
	if err != nil {
		klog.Fatal(err)
	}
	defer image.Close()

	if err != nil {
		klog.Fatal(err)
	}
	flags := os.O_RDWR
	if strings.HasSuffix(fileName, "disk.img") {
		flags |= os.O_CREATE
	}
	f, err := os.OpenFile(fileName, flags, 0650)
	if err != nil {
		klog.Fatal(err)
	}
	defer f.Close()

	err = writeData(image, f, imageID, progressGague)
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
