package openstack

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/utils/openstack/clientconfig"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/openstack"
	resource "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	RegionName                  = "regionName"
	AuthType                    = "authType"
	Username                    = "username"
	UserID                      = "userID"
	Password                    = "password"
	ApplicationCredentialID     = "applicationCredentialID"
	ApplicationCredentialName   = "applicationCredentialName"
	ApplicationCredentialSecret = "applicationCredentialSecret"
	Token                       = "token"
	SystemScope                 = "systemScope"
	ProjectName                 = "projectName"
	ProjectID                   = "projectID"
	UserDomainName              = "userDomainName"
	UserDomainID                = "userDomainID"
	ProjectDomainName           = "projectDomainName"
	ProjectDomainID             = "projectDomainID"
	DomainName                  = "domainName"
	DomainID                    = "domainID"
	DefaultDomain               = "defaultDomain"
	InsecureSkipVerify          = "insecureSkipVerify"
	CACert                      = "cacert"
	EndpointAvailability        = "availability"
)

var supportedAuthTypes = map[string]clientconfig.AuthType{
	"password":              clientconfig.AuthPassword,
	"token":                 clientconfig.AuthToken,
	"applicationcredential": clientconfig.AuthV3ApplicationCredential,
}

// Client
type Client struct {
	*plancontext.Context
	provider            *gophercloud.ProviderClient
	identityService     *gophercloud.ServiceClient
	computeService      *gophercloud.ServiceClient
	imageService        *gophercloud.ServiceClient
	blockStorageService *gophercloud.ServiceClient
}

// Connect.
func (r *Client) connect() (err error) {

	authInfo := &clientconfig.AuthInfo{
		AuthURL:           r.Source.Provider.Spec.URL,
		ProjectName:       r.getStringFromSecret(ProjectName),
		ProjectID:         r.getStringFromSecret(ProjectID),
		UserDomainName:    r.getStringFromSecret(UserDomainName),
		UserDomainID:      r.getStringFromSecret(UserDomainID),
		ProjectDomainName: r.getStringFromSecret(ProjectDomainName),
		ProjectDomainID:   r.getStringFromSecret(ProjectDomainID),
		DomainName:        r.getStringFromSecret(DomainName),
		DomainID:          r.getStringFromSecret(DomainID),
		DefaultDomain:     r.getStringFromSecret(DefaultDomain),
		AllowReauth:       true,
	}

	var authType clientconfig.AuthType
	authType, err = r.authType()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	switch authType {
	case clientconfig.AuthPassword:
		authInfo.Username = r.getStringFromSecret(Username)
		authInfo.UserID = r.getStringFromSecret(UserID)
		authInfo.Password = r.getStringFromSecret(Password)
	case clientconfig.AuthToken:
		authInfo.Token = r.getStringFromSecret(Token)
	case clientconfig.AuthV3ApplicationCredential:
		authInfo.Username = r.getStringFromSecret(Username)
		authInfo.ApplicationCredentialID = r.getStringFromSecret(ApplicationCredentialID)
		authInfo.ApplicationCredentialName = r.getStringFromSecret(ApplicationCredentialName)
		authInfo.ApplicationCredentialSecret = r.getStringFromSecret(ApplicationCredentialSecret)
	}

	identityUrl, err := url.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	var TLSClientConfig *tls.Config
	if identityUrl.Scheme == "https" {
		if r.getBoolFromSecret(InsecureSkipVerify) {
			TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			cacert := []byte(r.getStringFromSecret(CACert))
			if len(cacert) == 0 {
				r.Log.Info("CA certificate was not provided,system CA cert pool is used")
			} else {
				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM(cacert)
				if !ok {
					err = liberr.New("CA certificate is malformed, failed to configure the CA cert pool")
					return
				}
				TLSClientConfig = &tls.Config{RootCAs: roots}
			}

		}
	}

	provider, err := openstack.NewClient(r.Source.Provider.Spec.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	provider.HTTPClient.Transport = &http.Transport{
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
	}

	clientOpts := &clientconfig.ClientOpts{
		AuthType: authType,
		AuthInfo: authInfo,
	}

	opts, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	err = openstack.Authenticate(provider, *opts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	r.provider = provider

	availability := gophercloud.AvailabilityPublic
	if a := r.getStringFromSecret(EndpointAvailability); a != "" {
		availability = gophercloud.Availability(a)

	}

	endpointOpts := gophercloud.EndpointOpts{
		Region:       r.getStringFromSecret(RegionName),
		Availability: availability,
	}

	identityService, err := openstack.NewIdentityV3(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.identityService = identityService

	computeService, err := openstack.NewComputeV2(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.computeService = computeService

	imageService, err := openstack.NewImageServiceV2(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.imageService = imageService

	blockStorageService, err := openstack.NewBlockStorageV3(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.blockStorageService = blockStorageService

	return
}

// AuthType.
func (r *Client) authType() (authType clientconfig.AuthType, err error) {
	if configuredAuthType := r.getStringFromSecret(AuthType); configuredAuthType == "" {
		authType = clientconfig.AuthPassword
	} else if supportedAuthType, found := supportedAuthTypes[configuredAuthType]; found {
		authType = supportedAuthType
	} else {
		err = liberr.New("unsupported authentication type", "authType", configuredAuthType)
	}
	return
}

func (r *Client) getStringFromSecret(key string) string {
	if value, found := r.Source.Secret.Data[key]; found {
		return string(value)
	}
	return ""
}

func (r *Client) getBoolFromSecret(key string) bool {
	if keyStr := r.getStringFromSecret(key); keyStr != "" {
		value, err := strconv.ParseBool(keyStr)
		if err != nil {
			return false
		}
		return value
	}
	return false
}

// Get the VM by ref.
func (r *Client) getVM(vmRef ref.Ref) (vm *servers.Server, err error) {
	if vmRef.ID == "" {
		err = liberr.Wrap(
			err,
			"VM lookup failed.",
			"vm",
			vmRef.String())
		return
	}
	vm, err = servers.Get(r.computeService, vmRef.ID).Extract()
	if err != nil {
		err = liberr.New(
			fmt.Sprintf(
				"VM %s source lookup failed",
				vmRef.String()))
		return
	}
	return
}

func (r *Client) IsNotFoundErr(err error) bool {
	switch liberr.Unwrap(err).(type) {
	case gophercloud.ErrResourceNotFound, gophercloud.ErrDefault404:
		return true
	default:
		return false
	}
}

// Power on the source VM.
func (r *Client) PowerOn(vmRef ref.Ref) error {
	vm, err := r.getVM(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return err
	}
	if vm.Status != model.VmStatusShutoff {
		return nil
	}
	return startstop.Start(r.computeService, vm.ID).ExtractErr()
}

// Power off the source VM.
func (r *Client) PowerOff(vmRef ref.Ref) error {
	vm, err := r.getVM(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return err
	}
	if vm.Status == model.VmStatusShutoff {
		return nil
	}
	return startstop.Stop(r.computeService, vm.ID).ExtractErr()
}

// Return the source VM's power state.
func (r *Client) PowerState(vmRef ref.Ref) (string, error) {
	vm, err := r.getVM(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return "", err
	}
	return vm.Status, nil
}

// Return whether the source VM is powered off.
func (r *Client) PoweredOff(vmRef ref.Ref) (bool, error) {
	powerState, err := r.PowerState(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return false, err
	}
	return powerState == model.VmStatusShutoff, nil
}

// Create a snapshot of the source VM.
func (c *Client) CreateSnapshot(vmRef ref.Ref) (string, error) {
	return "", nil
}

// Remove all warm migration snapshots.
func (c *Client) RemoveSnapshots(vmRef ref.Ref, precopies []planapi.Precopy) error {
	return nil
}

// Check if a snapshot is ready to transfer.
func (c *Client) CheckSnapshotReady(vmRef ref.Ref, snapshot string) (bool, error) {
	return true, nil
}

// Set DataVolume checkpoints.
func (r *Client) SetCheckpoints(vmRef ref.Ref, precopies []planapi.Precopy, datavolumes []cdi.DataVolume, final bool) error {
	return nil
}

// Close connections to the provider API.
func (r *Client) Close() {
}

func (r *Client) Finalize(vms []*planapi.VMStatus, migrationName string) {
	for _, vm := range vms {
		vmResource := &resource.VM{}
		err := r.Source.Inventory.Find(vmResource, ref.Ref{ID: vm.Ref.ID})
		if err != nil {
			r.Log.Error(err, "Failed to find vm", "vm", vm.Name)
			return
		}

		for _, av := range vmResource.AttachedVolumes {
			lookupName := fmt.Sprintf("%s-%s", migrationName, av.ID)
			// In a normal operation the snapshot and volume should already have been removed
			// but they may remain in case of failure or cancellation of the migration

			// Delete snapshot
			snapshot := &resource.Snapshot{}
			err := r.Source.Inventory.Find(snapshot, ref.Ref{Name: lookupName})
			if err != nil {
				r.Log.Info("Failed to find snapshot", "snapshot", lookupName)
			} else {
				err = snapshots.Delete(r.blockStorageService, snapshot.ID).ExtractErr()
				if err != nil {
					r.Log.Error(err, "error removing snapshot", "snapshot", snapshot.ID)
				}
			}

			// Delete cloned volume
			volume := &resource.Volume{}
			err = r.Source.Inventory.Find(volume, ref.Ref{Name: lookupName})
			if err != nil {
				r.Log.Info("Failed to find volume", "volume", lookupName)
			} else {
				err = volumes.Delete(r.blockStorageService, volume.ID, volumes.DeleteOpts{Cascade: true}).ExtractErr()
				if err != nil {
					r.Log.Error(err, "error removing volume", "volume", volume.ID)
				}
			}

			// Delete Image
			image := &resource.Image{}
			err = r.Source.Inventory.Find(image, ref.Ref{Name: lookupName})
			if err != nil {
				r.Log.Info("Failed to find image", "image", lookupName)
			} else {
				err = images.Delete(r.imageService, image.ID).ExtractErr()
				if err != nil {
					r.Log.Error(err, "error removing image", "image", image.ID)
				}
			}
		}
	}
}

func (r *Client) DetachDisks(vmRef ref.Ref) (err error) {
	// no-op
	return
}
