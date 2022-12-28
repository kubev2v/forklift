package ovirt

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	liburl "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
	ovirtsdk "github.com/ovirt/go-ovirt"
	core "k8s.io/api/core/v1"
)

// Not found error.
type NotFound struct {
}

func (e *NotFound) Error() string {
	return "not found."
}

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

func TestDownloadOvfStore(secret *core.Secret, log logr.Logger) (validCA bool, err error) {
	// Create the connection to the server:
	conn, err := ovirtsdk.NewConnectionBuilder().
		URL(string(secret.Data["url"])).
		Username(string(secret.Data["user"])).
		Password(string(secret.Data["password"])).
		Insecure(true).
		Compress(true).
		Timeout(time.Second * 30).
		Build()
	if err != nil {
		log.Error(err, "Failed establishing connection to oVirt")
		err = liberr.Wrap(err)
		return
	}
	defer conn.Close()

	//Find OVF STORE ID
	disksService := conn.SystemService().DisksService()
	disksResponse, err := disksService.List().Send()
	if err != nil {
		log.Error(err, "Failed to get disks list from oVirt")
		err = liberr.Wrap(err)
		return
	}

	var ovfStoreId string
	if disks, ok := disksResponse.Disks(); ok {
		//get OVF STORE ID
		for _, disk := range disks.Slice() {
			if diskName, ok := disk.Name(); ok {
				if diskName == "OVF_STORE" {
					if diskId, ok := disk.Id(); ok {
						ovfStoreId = diskId
						break
					}
				}
			}
		}
	}
	// In case no OVF store was found in the system
	if ovfStoreId == "" {
		log.Error(err, "OVF Store does not exist in the system, CA certificate test can't performed")
		return
	}

	//in case the OVF STORE is locked wait for OK status before starting the image transfer
	conn.WaitForDisk(ovfStoreId, ovirtsdk.DISKSTATUS_OK, 20*time.Second)

	// Prepare image transfer request
	transfersService := conn.SystemService().ImageTransfersService()
	transfer := transfersService.Add()
	image, err := ovirtsdk.NewImageBuilder().Id(ovfStoreId).Build()
	if err != nil {
		log.Error(err, "Failed to build image for image transfer")
		err = liberr.Wrap(err)
		return
	}
	imageTransfer, err := ovirtsdk.NewImageTransferBuilder().Image(
		image,
	).Direction(
		ovirtsdk.IMAGETRANSFERDIRECTION_DOWNLOAD,
	).Build()
	if err != nil {
		log.Error(err, "Failed to build image transfer")
		err = liberr.Wrap(err)
		return
	}

	// Initialize image transfer and lock the disk
	transferResponse, err := transfer.ImageTransfer(imageTransfer).Send()
	if err != nil {
		log.Error(err, "Failed initialize image transfer")
		err = liberr.Wrap(err)
		return
	}
	currImageTransfer, ok := transferResponse.ImageTransfer()

	imageTransferId, ok := currImageTransfer.Id()

	for {
		if currImageTransfer.MustPhase() == ovirtsdk.IMAGETRANSFERPHASE_INITIALIZING {
			time.Sleep(1 * time.Second)
			currImagetransferResponse, errIT := conn.SystemService().ImageTransfersService().ImageTransferService(imageTransferId).Get().Send()
			if err != nil {
				log.Error(err, "Failed to send image transfer")
				err = liberr.Wrap(errIT)
				return
			}
			currImageTransfer, ok = currImagetransferResponse.ImageTransfer()
		} else {
			break
		}
	}

	caCert := secret.Data["cacert"]
	caCertPool := x509.NewCertPool()
	ok = caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		log.Error(err, "Failed to append certificate")
		err = liberr.Wrap(err)
		return
	}

	// Create http client
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	// Prepare disk GET request
	transferURL, ok := currImageTransfer.TransferUrl()
	req, err := http.NewRequest("GET", transferURL, nil)

	// Run the request
	resp, err := client.Do(req)
	if err != nil {
		// CA certificate is missing, request for image transfer url won't be sent
		if strings.Contains(err.Error(), "x509") {
			log.Info("Missing engine CA certificate, the request would not proceed")
			_, _ = conn.SystemService().ImageTransfersService().ImageTransferService(imageTransferId).Finalize().Send()
			ok = false
			return
		} else {
			log.Error(err, "Failed to send request to server")
			err = liberr.Wrap(err)
			return
		}
	}
	defer resp.Body.Close()

	//close image transfer
	_, err = conn.SystemService().ImageTransfersService().ImageTransferService(imageTransferId).Finalize().Send()
	if err != nil {
		log.Info("Failed to Finalize image transfer")
		err = liberr.Wrap(err)
		return
	}

	// Check server response
	if resp.StatusCode == http.StatusOK {
		log.Info("Response OK, CA certificate configured correct")
		validCA = true
		return
	}
	return
}
