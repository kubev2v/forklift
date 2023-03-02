package main

import (
	"flag"
	"io"
	"os"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
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
	kind       = "OpenstackVolumePopulator"
	resource   = "openstackvolumepopulators"
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

	populate(crName, crNamespace, volumePath, identityEndpoint, secretName, imageID)
}

type openstackConfig struct {
	username    string
	password    string
	domainName  string
	projectName string
	insecure    string
	region      string
}

func loadConfig(secretName, endpoint, namespace string) openstackConfig {
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
	region, err := os.ReadFile("/etc/secret-volume/region")
	if err != nil {
		klog.Fatal(err.Error())
	}
	domainName, err := os.ReadFile("/etc/secret-volume/domainName")
	if err != nil {
		klog.Fatal(err.Error())
	}
	insecure, err := os.ReadFile("/etc/secret-volume/insecure")
	if err != nil {
		klog.Fatal(err.Error())
	}

	return openstackConfig{
		username:    string(username),
		password:    string(password),
		insecure:    string(insecure),
		projectName: string(projectName),
		region:      string(region),
		domainName:  string(domainName),
	}
}

func populate(crName, crNamespace, fileName, endpoint, secretName, imageID string) {
	config := loadConfig(secretName, endpoint, crNamespace)

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: endpoint,
		DomainName:       config.domainName,
		Username:         config.username,
		Password:         config.password,
		TenantName:       config.projectName,
	}

	provider, err := openstack.AuthenticatedClient(authOpts)
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

	err = writeData(image, f, crName, crNamespace)
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
	klog.Info("Transferred: ", *cr.total)
	return n, err
}

func writeData(reader io.ReadCloser, file *os.File, crName, crNamespace string) error {
	// 	config, err := rest.InClusterConfig()
	// 	if err != nil {
	// 		klog.Fatal(err.Error())
	// 	}

	// 	client, err := dynamic.NewForConfig(config)
	// 	if err != nil {
	// 		klog.Fatal(err.Error())
	// 	}
	// 	gvr := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}

	total := new(int64)
	countingReader := CountingReader{reader, total}

	// TODO introduce /metrics endpoint to report progress
	// done := make(chan bool)
	// go func() {
	// 	for {
	// 		select {
	// 		case <-done:
	// 			return
	// 		default:
	// 			populatorCr, err := client.Resource(gvr).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
	// 			if err != nil {
	// 				klog.Fatal(err.Error())
	// 			}
	// 			status := map[string]interface{}{"transferred": fmt.Sprintf("%d", *total)}
	// 			unstructured.SetNestedField(populatorCr.Object, status, "status")

	// 			_, err = client.Resource(gvr).Namespace(crNamespace).Update(context.TODO(), populatorCr, metav1.UpdateOptions{})
	// 			if err != nil {
	// 				klog.Fatal(err.Error())
	// 			}
	// 		}

	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()

	if _, err := io.Copy(file, &countingReader); err != nil {
		klog.Fatal(err)
	}
	//done <- true
	// populatorCr, err := client.Resource(gvr).Namespace(crNamespace).Get(context.TODO(), crName, metav1.GetOptions{})
	// if err != nil {
	// 	klog.Fatal(err.Error())
	// }
	// status := map[string]interface{}{"transferred": *countingReader.total}
	// unstructured.SetNestedField(populatorCr.Object, status, "status")

	// _, err = client.Resource(gvr).Namespace(crNamespace).Update(context.TODO(), populatorCr, metav1.UpdateOptions{})
	// if err != nil {
	// 	klog.Error(err)
	// }

	return nil
}
