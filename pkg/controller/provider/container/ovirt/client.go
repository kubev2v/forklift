package ovirt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	liburl "net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	ovirtsdk "github.com/ovirt/go-ovirt"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//
// Not found error.
type NotFound struct {
}

func (e *NotFound) Error() string {
	return "not found."
}

//
// Client.
type Client struct {
	// Base URL.
	url string
	// Raw client.
	client *libweb.Client
	// Secret.
	secret                *core.Secret
	clientExpiration      time.Time
	clientTimeout         time.Duration
	accessTokenExpiration time.Time
	log                   logr.Logger
}

type ovirtTokenResponse struct {
	AccessToken string `json:"access_token"`
	Expiration  string `json:"exp"`
}

//
// Connect.
func (r *Client) connect() (status int, err error) {
	var TLSClientConfig *tls.Config

	if !r.clientExpiration.IsZero() && time.Now().After(r.clientExpiration) {
		r.log.Info("Recreating client, timeout exceeded")
		r.client = nil
	}

	if !r.accessTokenExpiration.IsZero() && time.Now().After(r.accessTokenExpiration) {
		r.log.Info("Recreating client, token expired")
		r.client = nil
	}

	if r.client != nil {
		return
	}

	if GetInsecureSkipVerifyFlag(r.secret) {
		TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		cacert := r.secret.Data["cacert"]
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			err = liberr.New("failed to parse cacert")
			return
		}
		TLSClientConfig = &tls.Config{RootCAs: roots}
	}

	r.url = strings.TrimRight(r.url, "/")
	client := &libweb.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 10 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       TLSClientConfig,
		},
	}

	url, err := liburl.Parse(r.url)
	if err != nil {
		return
	}

	url.Path = "/ovirt-engine/sso/oauth/token"
	values := url.Query()
	values.Add("grant_type", "password")
	values.Add("username", string(r.secret.Data["user"]))
	values.Add("password", string(r.secret.Data["password"]))
	values.Add("scope", "ovirt-app-api")

	client.Header = http.Header{
		"Accept": []string{"application/json"},
	}

	response := &ovirtTokenResponse{}
	url.RawQuery = values.Encode()
	status, err = client.Get(url.String(), response)
	if err != nil {
		return
	}

	// Providing bad credentials when requesting the token results
	// in 400, and not 401. So checking for != 200 instead
	if status != http.StatusOK {
		err = liberr.New("Request for token failed", "status", status)
		return
	}

	// Set the access token we received
	client.Header = http.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer " + response.AccessToken},
		"Version":       []string{"4"},
	}

	r.client = client
	r.clientExpiration = time.Now().Add(r.clientTimeout)

	expiration, err := strconv.ParseInt(response.Expiration, 10, 64)
	if err != nil {
		err = liberr.New("Failed to convert expiration time to integer", "Expiration", response.Expiration)
		return
	}

	r.accessTokenExpiration = time.Now().Local().Add(time.Duration(expiration))

	return
}

//
// List collection.
func (r *Client) list(path string, list interface{}, param ...libweb.Param) (err error) {
	url, err := liburl.Parse(r.url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path += "/" + path
	status, err := r.client.Get(url.String(), list, param...)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		err = liberr.New(http.StatusText(status))
		return
	}

	return
}

//
// Get a resource.
func (r *Client) get(path string, object interface{}, param ...libweb.Param) (err error) {
	url, err := liburl.Parse(r.url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	url.Path = path
	defer func() {
		if err != nil {
			err = liberr.Wrap(err, "url", url.String())
		}
	}()
	status, err := r.client.Get(url.String(), object, param...)
	if err != nil {
		return
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		err = &NotFound{}
	default:
		err = liberr.New(http.StatusText(status))
	}

	return
}

//
// Get system.
func (r *Client) system() (s *System, status int, err error) {
	status, err = r.connect()
	if err != nil {
		return
	}
	system := &System{}
	status, err = r.client.Get(r.url, system)
	if err != nil {
		return
	}
	return
}

//
// GetInsecureSkipVerifyFlag gets the insecureSkipVerify boolean flag
// value from the ovirt connection secret.
func GetInsecureSkipVerifyFlag(secret *core.Secret) bool {
	insecure, found := secret.Data["insecureSkipVerify"]
	if !found {
		return false
	}

	insecureSkipVerify, err := strconv.ParseBool(string(insecure))
	if err != nil {
		return false
	}

	return insecureSkipVerify
}

func DownloadOVFTestConection(secret *core.Secret) (ok bool, err error) {

	var diskIdOVF string

	url := fmt.Sprint(string(secret.Data["url"]), "/ovirt-engine/api")

	// Create the connection to the server:
	conn, err := ovirtsdk.NewConnectionBuilder().
		URL(url).
		Username(string(secret.Data["username"])).
		Password(string(secret.Data["password"])).
		Insecure(true).
		Compress(true).
		Timeout(time.Second * 10).
		Build()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer conn.Close()

	//Find OVF disk ID
	disksService := conn.SystemService().DisksService()
	diskResponse, err := disksService.List().Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	disks, ok := diskResponse.Disks()

	if ok {
		//get OVF disk ID
		for _, disk := range disks.Slice() {
			if diskName, ok := disk.Name(); ok {
				if diskName == "OVF_STORE" {
					if diskId, ok := disk.Id(); ok {
						diskIdOVF = diskId
					}
					break
				}
			}
		}
	}

	// Get a disk identified by uuid
	diskService := disksService.DiskService(diskIdOVF)
	diskRequest := diskService.Get()
	diskRequestSent, err := diskRequest.Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	disk, ok := diskRequestSent.Disk()

	diskId, ok := disk.Id()

	//in case the OVF disk is locked wait fo OK status befor stating the image transfer
	conn.WaitForDisk(diskId, ovirtsdk.DISKSTATUS_OK, 90*time.Second)

	// Prepare image transfer request
	transfersService := conn.SystemService().ImageTransfersService()
	transfer := transfersService.Add()
	image, err := ovirtsdk.NewImageBuilder().Id(diskId).Build()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	imageTransfer, err := ovirtsdk.NewImageTransferBuilder().Image(
		image,
	).Direction(
		ovirtsdk.IMAGETRANSFERDIRECTION_DOWNLOAD,
	).Build()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Initialize image transfer and lock the disk
	transfer.ImageTransfer(imageTransfer)
	transferReq, err := transfer.Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	currImageTransfer, ok := transferReq.ImageTransfer()

	imageTransferId, ok := currImageTransfer.Id()

	for {
		if currImageTransfer.MustPhase() == ovirtsdk.IMAGETRANSFERPHASE_INITIALIZING {
			time.Sleep(1 * time.Second)
			currImageTransferReq, errIT := conn.SystemService().ImageTransfersService().ImageTransferService(imageTransferId).Get().Send()
			if err != nil {
				err = liberr.Wrap(errIT)
				return
			}
			currImageTransfer, ok = currImageTransferReq.ImageTransfer()
		} else {
			break
		}
	}

	caCert := secret.Data["cacert"]
	caCertPool := x509.NewCertPool()
	ok = caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		err = liberr.Wrap(err)
		return
	}

	// Create http client
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	itURL, ok := currImageTransfer.ProxyUrl()

	// creat regex expresions to filter the proxy URL
	reInit := regexp.MustCompile(`https://`)
	reUrl := regexp.MustCompile(`^[^/]+`)

	transferURL := reInit.ReplaceAllString(itURL, "")
	transferURL = reUrl.FindString(transferURL)
	transferURL = fmt.Sprint("https://", transferURL, "/info/")

	// Prepare disk GET request
	req, err := http.NewRequest("GET", transferURL, nil)

	// Run the request
	resp, err := client.Do(req)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer resp.Body.Close()

	//close image transfer
	_, err = conn.SystemService().ImageTransfersService().ImageTransferService(imageTransferId).Cancel().Send()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Check server response
	if resp.StatusCode != http.StatusOK {
		//retrive missing CA cert and update secrete
		caUrl := fmt.Sprint(string(secret.Data["url"]), "ovirt-engine/services/pki-resource?resource=ca-certificate&format=X509-PEM-CA")
		cmd := exec.Command("wget", "--no-check-certificate", "-O", "ca.pem", caUrl)
		err = cmd.Run()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		data, errFile := ioutil.ReadFile("ca.pem")
		if err != nil {
			err = liberr.Wrap(errFile)
			return
		}

		updatedCa := fmt.Sprint(string(secret.Data["caCert"]), "/n", string(data))

		//create k8s client
		config, errClient := rest.InClusterConfig()
		if err != nil {
			err = liberr.Wrap(errClient)
			return
		}

		clientset, errConfig := kubernetes.NewForConfig(config)
		if err != nil {
			err = liberr.Wrap(errConfig)
			return
		}

		secretsClient := clientset.CoreV1().Secrets("forklift-konveyor")

		secret := &core.Secret{
			Data: map[string][]byte{
				"caCert":   []byte(updatedCa),
				"url":      []byte(string(secret.Data["url"])),
				"user":     []byte(string(secret.Data["user"])),
				"password": []byte(string(secret.Data["password"])),
			},
			Type: "Opaque",
		}
		_, err = secretsClient.Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		ok = false
		return
	}

	ok = true
	return

}

// func uploadDiskTestConection() bool {

// 	UID := "be18fdc8-85e3-11ed-a1eb-0242ac120002"

// 	inputRawURL := "https://vm-10-122.lab.eng.tlv2.redhat.com/ovirt-engine/api"
// 	url := "https://vm-10-122.lab.eng.tlv2.redhat.com/"

// 	caUrl := fmt.Sprint(url, "ovirt-engine/services/pki-resource?resource=ca-certificate&format=X509-PEM-CA")
// 	fmt.Println(caUrl)
// 	cmd := exec.Command("wget", "--no-check-certificate", "-O", "ca.pem", caUrl)
// 	err := cmd.Run()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Create the connection to the server:
// 	conn, err := ovirtsdk.NewConnectionBuilder().
// 		URL(inputRawURL).
// 		Username("admin@internal").
// 		Password("qum5net").
// 		Insecure(true).
// 		Compress(true).
// 		Timeout(time.Second * 10).
// 		Build()
// 	if err != nil {
// 		fmt.Printf("Make connection failed, reason: %v\n", err)
// 		return false
// 	}
// 	defer conn.Close()

// 	// Get the reference to the "clusters" service
// 	sdService := conn.SystemService().StorageDomainsService()

// 	sdResponse, err := sdService.List().Send()
// 	if err != nil {
// 		fmt.Printf("Failed to get cluster list, reason: %v\n", err)
// 		return false
// 	}

// 	sds, ok := sdResponse.StorageDomains()

// 	if ok {
// 		// Print the datacenter names and identifiers
// 		fmt.Printf("storage domains: (")
// 		for _, sd := range sds.Slice() {
// 			if sdName, ok := sd.Name(); ok {
// 				fmt.Println(" name: ", sdName)
// 			}
// 			if sdId, ok := sd.Id(); ok {
// 				fmt.Println(" id: ", sdId)
// 			}
// 		}
// 		fmt.Println(")")
// 	}

// 	diskBuilder := ovirtsdk.NewDiskBuilder().
// 		Name("test_conn_disk2").
// 		Id(UID).
// 		Format(ovirtsdk.DISKFORMAT_COW).
// 		ProvisionedSize(4096).
// 		StorageDomainsOfAny(
// 			ovirtsdk.NewStorageDomainBuilder().
// 				Id(sds.Slice()[0].MustId()).
// 				MustBuild())
// 	disk, err := diskBuilder.Build()
// 	if err != nil {
// 		fmt.Printf("error: %v", err)
// 		return false
// 	}

// 	_, err = conn.SystemService().DisksService().Add().Disk(disk).Send()
// 	if err != nil {
// 		fmt.Printf("error is %v\n", err)
// 	}

// 	conn.WaitForDisk(UID, ovirtsdk.DISKSTATUS_OK, 90*time.Second)

// 	//init image transfer

// 	transfersService := conn.SystemService().ImageTransfersService()
// 	transfer := transfersService.Add()
// 	imageTransfer := ovirtsdk.NewImageTransferBuilder().Image(
// 		ovirtsdk.NewImageBuilder().Id(disk.MustId()).MustBuild(),
// 	).Direction(
// 		ovirtsdk.IMAGETRANSFERDIRECTION_UPLOAD,
// 	).MustBuild()

// 	// Initialize image transfer and lock the disk
// 	transfer.ImageTransfer(imageTransfer)
// 	it := transfer.MustSend().MustImageTransfer()
// 	for {
// 		if it.MustPhase() == ovirtsdk.IMAGETRANSFERPHASE_INITIALIZING {
// 			time.Sleep(1 * time.Second)
// 			it = conn.SystemService().ImageTransfersService().ImageTransferService(it.MustId()).Get().MustSend().MustImageTransfer()
// 		} else {
// 			break
// 		}
// 	}

// 	// caCert, err := ioutil.ReadFile("ca.pem")
// 	// if err != nil {
// 	// 	fmt.Printf("Reading ca file failed, reason: %v\n", err)
// 	// 	return false
// 	// }
// 	caCert := secret.Data["cacert"]
// 	caCertPool := x509.NewCertPool()
// 	ok = caCertPool.AppendCertsFromPEM(caCert)
// 	if !ok {
// 		err = liberr.New("failed to parse cacert")
// 		return false
// 	}

// 	// Create http client
// 	tlsConfig := &tls.Config{
// 		RootCAs: caCertPool,
// 	}
// 	transport := &http.Transport{TLSClientConfig: tlsConfig}
// 	client := &http.Client{Transport: transport}

// 	infoUrl, ok := it.TransferUrl()
// 	newUrl := fmt.Sprint(infoUrl, "/info")
// 	fmt.Println(infoUrl)
// 	if ok != true {
// 		fmt.Println("error getting transfer url")
// 		return false
// 	}

// 	// add the url with /info in the end and check status
// 	req, err := http.NewRequest("GET", newUrl, nil)
// 	//req.Header.Set("Authorization", it.MustSignedTicket())

// 	// Run the request
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		fmt.Printf("Sending request failed, reason: %v\n", err)
// 		return false
// 	}
// 	defer resp.Body.Close()

// 	// Check server response
// 	if resp.StatusCode != http.StatusOK {
// 		fmt.Printf("bad status: %s", resp.Status)
// 		return false
// 	}

// 	//close image io transfer
// 	//

// 	disksService := conn.SystemService().DisksService()
// 	_, err = disksService.DiskService(UID).Remove().Send()
// 	if err != nil {
// 		fmt.Printf("Delete disk failed, reason: %v\n", err)
// 		return false

// 	}

// 	return true
// }
