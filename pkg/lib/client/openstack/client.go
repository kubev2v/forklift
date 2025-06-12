package openstack

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/volumeactions"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/applicationcredentials"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/regions"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/users"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/imagedata"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
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

// Client struct
type Client struct {
	URL                 string
	Options             map[string]string
	Log                 logging.LevelLogger
	provider            *gophercloud.ProviderClient
	identityService     *gophercloud.ServiceClient
	computeService      *gophercloud.ServiceClient
	imageService        *gophercloud.ServiceClient
	networkService      *gophercloud.ServiceClient
	blockStorageService *gophercloud.ServiceClient
}

func (c *Client) LoadOptionsFromSecret(secret *core.Secret) {
	c.Options = make(map[string]string)
	for key, value := range secret.Data {
		c.Options[key] = string(value)
	}
}

// Authenticate.
func (c *Client) Authenticate() (err error) {
	if c.provider != nil {
		return
	}
	authInfo := &clientconfig.AuthInfo{
		AuthURL:           c.URL,
		ProjectName:       c.getStringFromOptions(ProjectName),
		ProjectID:         c.getStringFromOptions(ProjectID),
		UserDomainName:    c.getStringFromOptions(UserDomainName),
		UserDomainID:      c.getStringFromOptions(UserDomainID),
		ProjectDomainName: c.getStringFromOptions(ProjectDomainName),
		ProjectDomainID:   c.getStringFromOptions(ProjectDomainID),
		DomainName:        c.getStringFromOptions(DomainName),
		DomainID:          c.getStringFromOptions(DomainID),
		DefaultDomain:     c.getStringFromOptions(DefaultDomain),
		AllowReauth:       true,
	}

	var authType clientconfig.AuthType
	authType, err = c.authType()
	if err != nil {
		return
	}
	switch authType {
	case clientconfig.AuthPassword:
		authInfo.Username = c.getStringFromOptions(Username)
		authInfo.UserID = c.getStringFromOptions(UserID)
		authInfo.Password = c.getStringFromOptions(Password)
	case clientconfig.AuthToken:
		authInfo.Token = c.getStringFromOptions(Token)
	case clientconfig.AuthV3ApplicationCredential:
		authInfo.Username = c.getStringFromOptions(Username)
		authInfo.ApplicationCredentialID = c.getStringFromOptions(ApplicationCredentialID)
		authInfo.ApplicationCredentialName = c.getStringFromOptions(ApplicationCredentialName)
		authInfo.ApplicationCredentialSecret = c.getStringFromOptions(ApplicationCredentialSecret)
	}

	var TLSConfig *tls.Config
	TLSConfig, err = c.getTLSConfig()
	if err != nil {
		c.Log.Error(err, "retrieving the TLS configuration")
		return
	}

	provider, err := openstack.NewClient(c.URL)
	if err != nil {
		c.Log.Error(err, "error creating new openstack client", "url", c.URL)
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
		TLSClientConfig:       TLSConfig,
	}

	clientOpts := &clientconfig.ClientOpts{
		AuthType: authType,
		AuthInfo: authInfo,
	}
	opts, err := clientconfig.AuthOptions(clientOpts)
	if err != nil {
		c.Log.Error(err, "error getting the AuthOptions from the ClientOpt", "clientOpts", clientOpts)
		return
	}
	err = openstack.Authenticate(provider, *opts)
	if err != nil {
		c.Log.Error(err, "error authenticating with the openstack provider", "provider", provider, "options", opts)
		return
	}
	c.provider = provider
	return
}

func (c *Client) getTLSConfig() (tlsConfig *tls.Config, err error) {
	identityUrl, err := url.Parse(c.URL)
	if err != nil {
		c.Log.Error(err, "error parsing the URL", "url", c.URL)
		return
	}
	if identityUrl.Scheme == "https" {
		if c.getBoolFromOptions(InsecureSkipVerify) {
			tlsConfig = &tls.Config{InsecureSkipVerify: true}
		} else {
			cacert := []byte(c.getStringFromOptions(CACert))
			if len(cacert) == 0 {
				c.Log.Info("CA certificate was not provided,system CA cert pool is used")
			} else {
				roots := x509.NewCertPool()
				ok := roots.AppendCertsFromPEM(cacert)
				if !ok {
					err = liberr.New("CA certificate is malformed, failed to configure the CA cert pool")
					return
				}
				tlsConfig = &tls.Config{RootCAs: roots}
			}
		}
	}
	return
}

// Connect
func (c *Client) Connect() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	userProjects := &[]Project{}
	err = c.GetUserProjects(userProjects)
	return
}

// AuthType.
func (c *Client) authType() (authType clientconfig.AuthType, err error) {
	if configuredAuthType := c.getStringFromOptions(AuthType); configuredAuthType == "" {
		authType = clientconfig.AuthPassword
	} else if supportedAuthType, found := supportedAuthTypes[configuredAuthType]; found {
		authType = supportedAuthType
	} else {
		err = liberr.New("unsupported authentication type", "authType", configuredAuthType)
	}
	return
}

func (c *Client) getEndpointOpts() (endpointOpts gophercloud.EndpointOpts) {
	endpointAvailability := gophercloud.AvailabilityPublic
	if availability := c.getStringFromOptions(EndpointAvailability); availability != "" {
		endpointAvailability = gophercloud.Availability(availability)

	}
	endpointOpts = gophercloud.EndpointOpts{
		Region:       c.getStringFromOptions(RegionName),
		Availability: endpointAvailability,
	}
	return
}

func (c *Client) getStringFromOptions(key string) string {
	if value, found := c.Options[key]; found {
		return value
	}
	return ""
}

func (c *Client) getBoolFromOptions(key string) bool {
	if keyStr := c.getStringFromOptions(key); keyStr != "" {
		value, err := strconv.ParseBool(keyStr)
		if err != nil {
			return false
		}
		return value
	}
	return false
}

func (r *Client) IsNotFound(err error) bool {
	switch unWrapErr := liberr.Unwrap(err).(type) {
	case gophercloud.ErrUnexpectedResponseCode:
		return unWrapErr.GetStatusCode() == http.StatusNotFound
	}
	return false
}

func (r *Client) IsForbidden(err error) bool {
	switch unWrapErr := liberr.Unwrap(err).(type) {
	case gophercloud.ErrUnexpectedResponseCode:
		return unWrapErr.GetStatusCode() == http.StatusForbidden
	}
	return false
}

// List Servers.
func (c *Client) List(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Region, *[]Project:
		err = c.identityServiceAPI(object, opts)
	case *[]Flavor, *[]VM:
		err = c.computeServiceAPI(object, opts)
	case *[]Image:
		err = c.imageServiceAPI(object, opts)
	case *[]Volume, *[]VolumeType, *[]Snapshot:
		err = c.blockStorageServiceAPI(object, opts)
	case *[]Network, *[]Subnet:
		err = c.networkServiceAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err, "trying to list objects", object, "opts", opts)
	}
	return
}

// Get a resource.
func (c *Client) Get(object interface{}, ID string) (err error) {
	switch object.(type) {
	case *Region, *Project:
		err = c.identityServiceAPI(object, &GetOpts{ID: ID})
	case *Flavor, *VM:
		err = c.computeServiceAPI(object, &GetOpts{ID: ID})
	case *Image:
		err = c.imageServiceAPI(object, &GetOpts{ID: ID})
	case *Volume, *VolumeType, *Snapshot:
		err = c.blockStorageServiceAPI(object, &GetOpts{ID: ID})
	case *Network, *Subnet:
		err = c.networkServiceAPI(object, &GetOpts{ID: ID})
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err, "trying to get object", object, "ID", ID)
	}
	return
}

// Create objects
func (c *Client) Create(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *Region, *Project:
		err = c.identityServiceAPI(object, opts)
	case *Flavor, *VM:
		err = c.computeServiceAPI(object, opts)
	case *Image:
		err = c.imageServiceAPI(object, opts)
	case *Volume, *VolumeType, *Snapshot:
		err = c.blockStorageServiceAPI(object, opts)
	case *Network, *Subnet:
		err = c.networkServiceAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err, "trying to create object", object, "opts", opts)
	}
	return
}

// Update
func (c *Client) Update(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *Region, *Project:
		err = c.identityServiceAPI(object, opts)
	case *Flavor, *VM:
		err = c.computeServiceAPI(object, opts)
	case *Image:
		err = c.imageServiceAPI(object, opts)
	case *Volume, *VolumeType, *Snapshot:
		err = c.blockStorageServiceAPI(object, opts)
	case *Network, *Subnet:
		err = c.networkServiceAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err, "trying to update object", object, "opts", opts)
	}
	return
}

// Delete a resource.
func (c *Client) Delete(object interface{}) (err error) {
	switch object.(type) {
	case *Region, *Project:
		err = c.identityServiceAPI(object, &DeleteOpts{})
	case *Flavor, *VM:
		err = c.computeServiceAPI(object, &DeleteOpts{})
	case *Image:
		err = c.imageServiceAPI(object, &DeleteOpts{})
	case *Volume, *VolumeType, *Snapshot:
		err = c.blockStorageServiceAPI(object, &DeleteOpts{})
	case *Network, *Subnet:
		err = c.networkServiceAPI(object, &DeleteOpts{})
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err, "trying to remove object", object)
	}
	return
}

func (c *Client) unsupportedTypeError(object interface{}) (err error) {
	err = liberr.New(fmt.Sprintf("unsupported type %T", object))
	return
}

func (c *Client) connectIdentityServiceAPI() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	if c.identityService == nil {
		var identityService *gophercloud.ServiceClient
		endpointOpts := c.getEndpointOpts()
		identityService, err = openstack.NewIdentityV3(c.provider, endpointOpts)
		if err != nil {
			c.Log.Error(err, "creating the identity service client", "provider", c.provider, "options", endpointOpts)
			return
		}
		c.identityService = identityService
	}
	return
}

func (c *Client) identityServiceAPI(object interface{}, opts interface{}) (err error) {
	err = c.connectIdentityServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch object.(type) {
	case *Region, *[]Region:
		err = c.regionAPI(object, opts)
	case *Project, *[]Project:
		err = c.projectAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) regionAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Region:
		object := object.(*[]Region)
		switch opts := opts.(type) {
		case *RegionListOpts:
			err = c.regionList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Region:
		object := object.(*Region)
		var region *regions.Region
		switch opts := opts.(type) {
		case *GetOpts:
			region, err = regions.Get(c.identityService, opts.ID).Extract()
			if err != nil {
				return
			}
			*object = Region{*region}
		case *RegionCreateOpts:
			region, err = regions.Create(c.identityService, opts).Extract()
			if err != nil {
				return
			}
			*object = Region{*region}
		case *RegionUpdateOpts:
			region, err = regions.Update(c.identityService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Region{*region}
		case *DeleteOpts:
			err = regions.Delete(c.identityService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) regionList(object *[]Region, opts *RegionListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = regions.List(c.identityService, opts).AllPages()
	if err != nil {
		return
	}
	var regionList []regions.Region
	regionList, err = regions.ExtractRegions(allPages)
	if err != nil {
		return
	}
	var instanceList []Region
	for _, region := range regionList {
		instanceList = append(instanceList, Region{region})
	}
	*object = instanceList
	return
}

func (c *Client) projectAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Project:
		object := object.(*[]Project)
		switch opts := opts.(type) {
		case *ProjectListOpts:
			err = c.projectList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Project:
		object := object.(*Project)
		var project *projects.Project
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			project, err = projects.Get(c.identityService, ID).Extract()
			if err != nil {
				return
			}
			*object = Project{*project}
		case *ProjectCreateOpts:
			project, err = projects.Create(c.identityService, opts).Extract()
			if err != nil {
				return
			}
			*object = Project{*project}
		case *ProjectUpdateOpts:
			project, err = projects.Update(c.identityService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Project{Project: *project}
		case *DeleteOpts:
			err = projects.Delete(c.identityService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) projectList(object *[]Project, opts *ProjectListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = projects.List(c.identityService, opts).AllPages()
	if err != nil {
		return
	}
	var projectList []projects.Project
	projectList, err = projects.ExtractProjects(allPages)
	if err != nil {
		return
	}
	var instanceList []Project
	for _, project := range projectList {
		instanceList = append(instanceList, Project{project})
	}
	*object = instanceList
	return
}

func (c *Client) connectComputeServiceAPI() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	if c.computeService == nil {
		var computeService *gophercloud.ServiceClient
		endpointOpts := c.getEndpointOpts()
		computeService, err = openstack.NewComputeV2(c.provider, endpointOpts)
		if err != nil {
			c.Log.Error(err, "creating the compute service client", "provider", c.provider, "options", endpointOpts)
			return
		}
		c.computeService = computeService
	}
	return
}

func (c *Client) computeServiceAPI(object interface{}, opts interface{}) (err error) {
	err = c.connectComputeServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch object.(type) {
	case *VM, *[]VM:
		err = c.vmAPI(object, opts)
	case *Flavor, *[]Flavor:
		err = c.flavorAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) vmAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]VM:
		object := object.(*[]VM)
		switch opts := opts.(type) {
		case *VMListOpts:
			err = c.vmList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *VM:
		object := object.(*VM)
		var server *servers.Server
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			server, err = servers.Get(c.computeService, ID).Extract()
			if err != nil {
				return
			}
			*object = VM{Server: *server}
		case *VMCreateOpts:
			server, err = servers.Create(c.computeService, opts).Extract()
			if err != nil {
				return
			}
			*object = VM{Server: *server}
		case *VMUpdateOpts:
			server, err = servers.Update(c.computeService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = VM{Server: *server}
		case *DeleteOpts:
			err = servers.Delete(c.computeService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) vmList(object *[]VM, opts *VMListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = servers.List(c.computeService, opts).AllPages()
	if err != nil {
		return
	}
	var serverList []servers.Server
	serverList, err = servers.ExtractServers(allPages)
	if err != nil {
		return
	}
	var instanceList []VM
	for _, server := range serverList {
		instanceList = append(instanceList, VM{server})
	}
	*object = instanceList
	return
}

func (c *Client) flavorAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Flavor:
		object := object.(*[]Flavor)
		switch opts := opts.(type) {
		case *FlavorListOpts:
			err = c.flavorList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Flavor:
		object := object.(*Flavor)
		var flavor *flavors.Flavor
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			flavor, err = flavors.Get(c.computeService, ID).Extract()
			if err != nil {
				return
			}
			var extraSpecs map[string]string
			extraSpecs, err = flavors.ListExtraSpecs(c.computeService, ID).Extract()
			if err != nil {
				return
			}
			*object = Flavor{Flavor: *flavor, ExtraSpecs: extraSpecs}
		case *FlavorCreateOpts:
			flavor, err = flavors.Create(c.computeService, opts).Extract()
			if err != nil {
				return
			}
			*object = Flavor{Flavor: *flavor}
		case *FlavorUpdateOpts:
			flavor, err = flavors.Update(c.computeService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Flavor{Flavor: *flavor}
		case *DeleteOpts:
			err = flavors.Delete(c.computeService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) flavorList(object *[]Flavor, opts *FlavorListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = flavors.ListDetail(c.computeService, opts).AllPages()
	if err != nil {
		return
	}
	var flavorList []flavors.Flavor
	flavorList, err = flavors.ExtractFlavors(allPages)
	if err != nil {
		return
	}
	var instanceList []Flavor
	var extraSpecs map[string]string
	for _, flavor := range flavorList {
		extraSpecs, err = flavors.ListExtraSpecs(c.computeService, flavor.ID).Extract()
		if err != nil {
			return
		}
		instanceList = append(instanceList, Flavor{Flavor: flavor, ExtraSpecs: extraSpecs})
	}
	*object = instanceList
	return
}

func (c *Client) connectImageServiceAPI() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	if c.imageService == nil {
		var imageService *gophercloud.ServiceClient
		endpointOpts := c.getEndpointOpts()
		imageService, err = openstack.NewImageServiceV2(c.provider, endpointOpts)
		if err != nil {
			c.Log.Error(err, "creating the image service client", "provider", c.provider, "options", endpointOpts)
			return
		}
		c.imageService = imageService
	}
	return
}

func (c *Client) imageServiceAPI(object interface{}, opts interface{}) (err error) {
	err = c.connectImageServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch object.(type) {
	case *Image, *[]Image:
		err = c.imageAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) imageAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Image:
		object := object.(*[]Image)
		switch opts := opts.(type) {
		case *ImageListOpts:
			err = c.imageList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Image:
		object := object.(*Image)
		var image *images.Image
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			image, err = images.Get(c.imageService, ID).Extract()
			if err != nil {
				return
			}
			*object = Image{Image: *image}
		case *ImageCreateOpts:
			image, err = images.Create(c.imageService, opts).Extract()
			if err != nil {
				return
			}
			*object = Image{Image: *image}
		case *ImageUpdateOpts:
			image, err = images.Update(c.imageService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Image{Image: *image}
		case *DeleteOpts:
			err = images.Delete(c.imageService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) imageList(object *[]Image, opts *ImageListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = images.List(c.imageService, opts).AllPages()
	if err != nil {
		return
	}
	var imageList []images.Image
	imageList, err = images.ExtractImages(allPages)
	if err != nil {
		return
	}
	var instanceList []Image
	for _, image := range imageList {
		instanceList = append(instanceList, Image{image})
	}
	*object = instanceList
	return
}

func (c *Client) connectBlockStorageServiceAPI() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	if c.blockStorageService == nil {
		var blockStorageService *gophercloud.ServiceClient
		endpointOpts := c.getEndpointOpts()
		blockStorageService, err = openstack.NewBlockStorageV3(c.provider, endpointOpts)
		if err != nil {
			c.Log.Error(err, "creating the block storage service client", "provider", c.provider, "options", endpointOpts)
			return
		}
		c.blockStorageService = blockStorageService
	}
	return
}

func (c *Client) blockStorageServiceAPI(object interface{}, opts interface{}) (err error) {
	err = c.connectBlockStorageServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch object.(type) {
	case *Volume, *[]Volume:
		err = c.volumeAPI(object, opts)
	case *VolumeType, *[]VolumeType:
		err = c.volumeTypeAPI(object, opts)
	case *Snapshot, *[]Snapshot:
		err = c.snapshotAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) volumeAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Volume:
		object := object.(*[]Volume)
		switch opts := opts.(type) {
		case *VolumeListOpts:
			err = c.volumeList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Volume:
		object := object.(*Volume)
		var volume *volumes.Volume
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			volume, err = volumes.Get(c.blockStorageService, ID).Extract()
			if err != nil {
				return
			}
			*object = Volume{Volume: *volume}
		case *VolumeCreateOpts:
			volume, err = volumes.Create(c.blockStorageService, opts).Extract()
			if err != nil {
				return
			}
			*object = Volume{Volume: *volume}
		case *VolumeUpdateOpts:
			volume, err = volumes.Update(c.blockStorageService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Volume{Volume: *volume}
		case *DeleteOpts:
			err = volumes.Delete(c.blockStorageService, object.ID, volumes.DeleteOpts{Cascade: true}).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) volumeList(object *[]Volume, opts *VolumeListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = volumes.List(c.blockStorageService, opts).AllPages()
	if err != nil {
		return
	}
	var volumeList []volumes.Volume
	volumeList, err = volumes.ExtractVolumes(allPages)
	if err != nil {
		return
	}
	var instanceList []Volume
	for _, volume := range volumeList {
		instanceList = append(instanceList, Volume{volume})
	}
	*object = instanceList
	return
}

func (c *Client) volumeTypeAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]VolumeType:
		object := object.(*[]VolumeType)
		switch opts := opts.(type) {
		case *VolumeTypeListOpts:
			err = c.volumeTypeList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *VolumeType:
		object := object.(*VolumeType)
		var volumeType *volumetypes.VolumeType
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			volumeType, err = volumetypes.Get(c.blockStorageService, ID).Extract()
			if err != nil {
				return
			}
			*object = VolumeType{VolumeType: *volumeType}
		case *VolumeTypeCreateOpts:
			volumeType, err = volumetypes.Create(c.blockStorageService, opts).Extract()
			if err != nil {
				return
			}
			*object = VolumeType{VolumeType: *volumeType}
		case *VolumeTypeUpdateOpts:
			volumeType, err = volumetypes.Update(c.blockStorageService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = VolumeType{VolumeType: *volumeType}
		case *DeleteOpts:
			err = volumetypes.Delete(c.blockStorageService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) volumeTypeList(object *[]VolumeType, opts *VolumeTypeListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = volumetypes.List(c.blockStorageService, opts).AllPages()
	if err != nil {
		return
	}
	var volumetypeList []volumetypes.VolumeType
	volumetypeList, err = volumetypes.ExtractVolumeTypes(allPages)
	if err != nil {
		return
	}
	var instanceList []VolumeType
	for _, volumetype := range volumetypeList {
		instanceList = append(instanceList, VolumeType{volumetype})
	}
	*object = instanceList
	return
}

func (c *Client) snapshotAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Snapshot:
		object := object.(*[]Snapshot)
		switch opts := opts.(type) {
		case *SnapshotListOpts:
			err = c.snapshotList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Snapshot:
		object := object.(*Snapshot)
		var snapshot *snapshots.Snapshot
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			snapshot, err = snapshots.Get(c.blockStorageService, ID).Extract()
			if err != nil {
				return
			}
			*object = Snapshot{Snapshot: *snapshot}
		case *SnapshotCreateOpts:
			snapshot, err = snapshots.Create(c.blockStorageService, opts).Extract()
			if err != nil {
				return
			}
			*object = Snapshot{Snapshot: *snapshot}
		case *SnapshotUpdateOpts:
			snapshot, err = snapshots.Update(c.blockStorageService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Snapshot{Snapshot: *snapshot}
		case *DeleteOpts:
			err = snapshots.Delete(c.blockStorageService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) snapshotList(object *[]Snapshot, opts *SnapshotListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = snapshots.List(c.blockStorageService, opts).AllPages()
	if err != nil {
		return
	}
	var snapshotList []snapshots.Snapshot
	snapshotList, err = snapshots.ExtractSnapshots(allPages)
	if err != nil {
		return
	}
	var instanceList []Snapshot
	for _, snapshot := range snapshotList {
		instanceList = append(instanceList, Snapshot{snapshot})
	}
	*object = instanceList
	return
}

func (c *Client) connectNetworkServiceAPI() (err error) {
	err = c.Authenticate()
	if err != nil {
		return
	}
	if c.networkService == nil {
		var networkService *gophercloud.ServiceClient
		endpointOpts := c.getEndpointOpts()
		networkService, err = openstack.NewNetworkV2(c.provider, endpointOpts)
		if err != nil {
			c.Log.Error(err, "creating the network service client", "provider", c.provider, "options", endpointOpts)
			return
		}
		c.networkService = networkService
	}
	return
}

func (c *Client) networkServiceAPI(object interface{}, opts interface{}) (err error) {
	err = c.connectNetworkServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	switch object.(type) {
	case *Network, *[]Network:
		err = c.networkAPI(object, opts)
	case *Subnet, *[]Subnet:
		err = c.subnetAPI(object, opts)
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) networkAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Network:
		object := object.(*[]Network)
		switch opts := opts.(type) {
		case *NetworkListOpts:
			err = c.networkList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Network:
		object := object.(*Network)
		var network *networks.Network
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			network, err = networks.Get(c.networkService, ID).Extract()
			if err != nil {
				return
			}
			*object = Network{Network: *network}
		case *NetworkCreateOpts:
			network, err = networks.Create(c.networkService, opts).Extract()
			if err != nil {
				return
			}
			*object = Network{Network: *network}
		case *NetworkUpdateOpts:
			network, err = networks.Update(c.networkService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Network{Network: *network}
		case *DeleteOpts:
			err = networks.Delete(c.networkService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) networkList(object *[]Network, opts *NetworkListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = networks.List(c.networkService, opts).AllPages()
	if err != nil {
		return
	}
	var networkList []networks.Network
	networkList, err = networks.ExtractNetworks(allPages)
	if err != nil {
		return
	}
	var instanceList []Network
	for _, network := range networkList {
		instanceList = append(instanceList, Network{network})
	}
	*object = instanceList
	return
}

func (c *Client) subnetAPI(object interface{}, opts interface{}) (err error) {
	switch object.(type) {
	case *[]Subnet:
		object := object.(*[]Subnet)
		switch opts := opts.(type) {
		case *SubnetListOpts:
			err = c.subnetList(object, opts)
		default:
			err = c.unsupportedTypeError(object)
		}
	case *Subnet:
		object := object.(*Subnet)
		var subnet *subnets.Subnet
		switch opts := opts.(type) {
		case *GetOpts:
			ID := opts.ID
			subnet, err = subnets.Get(c.networkService, ID).Extract()
			if err != nil {
				return
			}
			*object = Subnet{Subnet: *subnet}
		case *SubnetCreateOpts:
			subnet, err = subnets.Create(c.networkService, opts).Extract()
			if err != nil {
				return
			}
			*object = Subnet{Subnet: *subnet}
		case *SubnetUpdateOpts:
			subnet, err = subnets.Update(c.networkService, object.ID, opts).Extract()
			if err != nil {
				return
			}
			*object = Subnet{Subnet: *subnet}
		case *DeleteOpts:
			err = subnets.Delete(c.networkService, object.ID).ExtractErr()
		default:
			err = c.unsupportedTypeError(object)
		}
	default:
		err = c.unsupportedTypeError(object)
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) subnetList(object *[]Subnet, opts *SubnetListOpts) (err error) {
	var allPages pagination.Page
	allPages, err = subnets.List(c.networkService, opts).AllPages()
	if err != nil {
		return
	}
	var subnetList []subnets.Subnet
	subnetList, err = subnets.ExtractSubnets(allPages)
	if err != nil {
		return
	}
	var instanceList []Subnet
	for _, subnet := range subnetList {
		instanceList = append(instanceList, Subnet{subnet})
	}
	*object = instanceList
	return
}

func (c *Client) GetUserProjects(userProjects *[]Project) (err error) {
	var userID string
	var allPages pagination.Page
	err = c.connectIdentityServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	userID, err = c.getAuthenticatedUserID()
	if err != nil {
		return
	}
	allPages, err = users.ListProjects(c.identityService, userID).AllPages()
	if err != nil {
		return
	}
	var projectList []projects.Project
	projectList, err = projects.ExtractProjects(allPages)
	if err != nil {
		return
	}
	var instanceList []Project
	for _, project := range projectList {
		instanceList = append(instanceList, Project{project})
	}
	*userProjects = instanceList
	return
}

func (c *Client) GetClientRegion(region *Region) (err error) {
	if regionID, ok := c.Options[RegionName]; ok {
		err = c.Get(region, regionID)
	} else {
		err = liberr.New("no region name found within the client options")
	}
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (c *Client) getAuthenticatedUserID() (userID string, err error) {
	if c.provider == nil {
		err = c.Authenticate()
		if err != nil {
			return
		}
	}
	authResult := c.provider.GetAuthResult()
	if authResult == nil {
		//ProviderClient did not use openstack.Authenticate(), e.g. because token
		//was set manually with ProviderClient.SetToken()
		err = liberr.New("no AuthResult available")
		return
	}
	switch authResultType := authResult.(type) {
	case tokens.CreateResult:
		var user *tokens.User
		user, err = authResultType.ExtractUser()
		if err != nil {
			return
		}
		userID = user.ID
		return
	default:
		err = c.unsupportedTypeError(authResultType)
		return

	}
}

func (c *Client) GetClientProject(project *Project) (err error) {
	var found bool
	var userProjects []Project

	err = c.GetUserProjects(&userProjects)
	if err != nil {
		return
	}
	projectName := c.getStringFromOptions(ProjectName)
	projectID := c.getStringFromOptions(ProjectID)

	if projectName == "" && projectID == "" {
		projectID, err = c.getProjectIDFromApplicationCredentials()
		if err != nil {
			return
		}
	}
	for _, userProject := range userProjects {
		if userProject.Name == projectName || userProject.ID == projectID {
			found = true
			*project = userProject
			break
		}
	}
	if !found {
		err = gophercloud.ErrDefault404{}
		return
	}
	return
}

func (c *Client) getProjectIDFromApplicationCredentials() (projectID string, err error) {
	err = c.connectIdentityServiceAPI()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var userID string
	userID, err = c.getAuthenticatedUserID()
	if err != nil {
		return
	}
	applicationCredentialID := c.getStringFromOptions(ApplicationCredentialID)
	if applicationCredentialID != "" {
		var applicationCredential *applicationcredentials.ApplicationCredential
		applicationCredential, err = applicationcredentials.Get(c.identityService, userID, applicationCredentialID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		projectID = applicationCredential.ProjectID
	}
	applicationCredentialName := c.getStringFromOptions(ApplicationCredentialName)
	if applicationCredentialName != "" {
		var applicationCredentials []applicationcredentials.ApplicationCredential
		var allPages pagination.Page
		allPages, err = applicationcredentials.List(c.identityService, userID, &applicationcredentials.ListOpts{Name: applicationCredentialName}).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		applicationCredentials, err = applicationcredentials.ExtractApplicationCredentials(allPages)
		projectID = applicationCredentials[0].ProjectID
	}
	return
}

// Return the source VM's power state.
func (c *Client) VMStatus(vmID string) (vmStatus string, err error) {
	vm := &VM{}
	err = c.Get(vm, vmID)
	if err != nil {
		return "", err
	}
	vmStatus = vm.Status
	return
}

// Power on the source VM.
func (c *Client) VMStart(vmID string) (err error) {
	err = c.connectComputeServiceAPI()
	if err != nil {
		return
	}
	err = startstop.Start(c.computeService, vmID).ExtractErr()
	return
}

// Power off the source VM.
func (c *Client) VMStop(vmID string) (err error) {
	err = c.connectComputeServiceAPI()
	if err != nil {
		return
	}
	err = startstop.Stop(c.computeService, vmID).ExtractErr()
	return
}

func (c *Client) VMCreateSnapshotImage(vmID string, opts VMCreateImageOpts) (image *Image, err error) {
	err = c.connectComputeServiceAPI()
	if err != nil {
		return
	}
	imageID, err := servers.CreateImage(c.computeService, vmID, opts).ExtractImageID()
	if err != nil {
		return nil, err
	}
	image = &Image{}
	err = c.Get(image, imageID)
	return
}

func (c *Client) VMGetSnapshotImages(opts *ImageListOpts) (images []Image, err error) {
	err = c.List(&images, opts)
	return
}

func (c *Client) VMRemoveSnapshotImage(imageID string) (err error) {
	image := &Image{}
	image.ID = imageID
	err = c.Delete(image)
	return
}

func (c *Client) UploadImage(name, volumeID string) (image *Image, err error) {
	err = c.connectBlockStorageServiceAPI()
	if err != nil {
		return
	}
	opts := volumeactions.UploadImageOpts{
		ImageName:  name,
		DiskFormat: "raw",
	}
	volumeImage, err := volumeactions.UploadImage(c.blockStorageService, volumeID, opts).Extract()
	if err != nil {
		return
	}
	image = &Image{}
	err = c.Get(image, volumeImage.ImageID)
	return
}

func (c *Client) DownloadImage(imageID string) (data io.ReadCloser, err error) {
	err = c.connectImageServiceAPI()
	if err != nil {
		return
	}
	data, err = imagedata.Download(c.imageService, imageID).Extract()
	return
}

func (c *Client) UnsetImageMetadata(volumeID, key string) (err error) {
	err = c.connectBlockStorageServiceAPI()
	if err != nil {
		return
	}
	err = volumeactions.UnsetImageMetadata(c.blockStorageService, volumeID, volumeactions.UnsetImageMetadataOpts{Key: key}).ExtractErr()
	return
}
