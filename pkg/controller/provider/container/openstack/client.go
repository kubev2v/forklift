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

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/regions"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/users"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
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
	Secret              *core.Secret
	provider            *gophercloud.ProviderClient
	identityService     *gophercloud.ServiceClient
	ComputeService      *gophercloud.ServiceClient
	ImageService        *gophercloud.ServiceClient
	NetworkService      *gophercloud.ServiceClient
	BlockStorageService *gophercloud.ServiceClient
	Log                 logr.Logger
}

// Authenticate.
func (r *Client) Authenticate() (err error) {

	authInfo := &clientconfig.AuthInfo{
		AuthURL:           r.URL,
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

	identityUrl, err := url.Parse(r.URL)
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

	provider, err := openstack.NewClient(r.URL)
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
	return
}

// Connect
func (r *Client) Connect() (err error) {
	if r.provider == nil {
		err = r.Authenticate()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

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
	r.ComputeService = computeService

	imageService, err := openstack.NewImageServiceV2(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.ImageService = imageService

	networkService, err := openstack.NewNetworkV2(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.NetworkService = networkService

	blockStorageService, err := openstack.NewBlockStorageV3(r.provider, endpointOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.BlockStorageService = blockStorageService

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
	if value, found := r.Secret.Data[key]; found {
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

// List Servers.
func (r *Client) list(object interface{}, listopts interface{}) (err error) {

	var allPages pagination.Page

	switch object.(type) {
	case *[]Region:
		object := object.(*[]Region)
		allPages, err = regions.List(r.identityService, listopts.(*RegionListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var regionList []regions.Region
		regionList, err = regions.ExtractRegions(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Region
		for _, region := range regionList {
			// TODO implement support multiple regions/projects sync per user
			if region.ID == r.getStringFromSecret(RegionName) {
				instanceList = append(instanceList, Region{region})
			}
		}
		*object = instanceList
		return

	case *[]Project:
		object := object.(*[]Project)
		// TODO implement support multiple regions/projects sync per user
		opts := listopts.(*ProjectListOpts)
		opts.Name = r.getStringFromSecret(ProjectName)
		allPages, err = projects.List(r.identityService, opts).AllPages()
		if err != nil {
			if !r.isForbidden(err) {
				err = liberr.Wrap(err)
				return
			}
			*object, err = r.getUserProjects()
			if err != nil {
				err = liberr.Wrap(err)
			}
			return
		}
		var projectList []projects.Project
		projectList, err = projects.ExtractProjects(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Project
		for _, project := range projectList {
			instanceList = append(instanceList, Project{project})
		}
		*object = instanceList
		return

	case *[]Flavor:
		object := object.(*[]Flavor)
		allPages, err = flavors.ListDetail(r.ComputeService, listopts.(*FlavorListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var flavorList []flavors.Flavor
		flavorList, err = flavors.ExtractFlavors(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Flavor
		var extraSpecs map[string]string
		for _, flavor := range flavorList {
			extraSpecs, err = flavors.ListExtraSpecs(r.ComputeService, flavor.ID).Extract()
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			instanceList = append(instanceList, Flavor{Flavor: flavor, ExtraSpecs: extraSpecs})
		}
		*object = instanceList
		return

	case *[]Image:
		object := object.(*[]Image)
		allPages, err = images.List(r.ImageService, listopts.(*ImageListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var imageList []images.Image
		imageList, err = images.ExtractImages(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Image
		for _, image := range imageList {
			instanceList = append(instanceList, Image{image})
		}
		*object = instanceList
		return

	case *[]VM:
		object := object.(*[]VM)
		allPages, err = servers.List(r.ComputeService, listopts.(*VMListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var serverList []servers.Server
		serverList, err = servers.ExtractServers(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []VM
		for _, server := range serverList {
			instanceList = append(instanceList, VM{server})
		}
		*object = instanceList
		return

	case *[]Snapshot:
		object := object.(*[]Snapshot)
		allPages, err = snapshots.List(r.BlockStorageService, nil).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var snapshotList []snapshots.Snapshot
		snapshotList, err = snapshots.ExtractSnapshots(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Snapshot
		for _, snapshot := range snapshotList {
			instanceList = append(instanceList, Snapshot{snapshot})
		}
		*object = instanceList
		return

	case *[]Volume:
		object := object.(*[]Volume)
		allPages, err = volumes.List(r.BlockStorageService, listopts.(*VolumeListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var volumeList []volumes.Volume
		volumeList, err = volumes.ExtractVolumes(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Volume
		for _, volume := range volumeList {
			instanceList = append(instanceList, Volume{volume})
		}
		*object = instanceList
		return

	case *[]VolumeType:
		object := object.(*[]VolumeType)
		allPages, err = volumetypes.List(r.BlockStorageService, listopts.(*VolumeTypeListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var volumeTypeList []volumetypes.VolumeType
		volumeTypeList, err = volumetypes.ExtractVolumeTypes(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []VolumeType
		for _, volumeType := range volumeTypeList {
			if volumeType.ExtraSpecs == nil {
				volumeType.ExtraSpecs = map[string]string{}
			}
			instanceList = append(instanceList, VolumeType{volumeType})
		}
		*object = instanceList
		return

	case *[]Network:
		object := object.(*[]Network)
		allPages, err = networks.List(r.NetworkService, listopts.(*NetworkListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var networkList []networks.Network
		networkList, err = networks.ExtractNetworks(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Network
		for _, network := range networkList {
			instanceList = append(instanceList, Network{network})
		}
		*object = instanceList
		return

	case *[]Subnet:
		object := object.(*[]Subnet)
		allPages, err = subnets.List(r.NetworkService, listopts.(*SubnetListOpts)).AllPages()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var subnetList []subnets.Subnet
		subnetList, err = subnets.ExtractSubnets(allPages)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var instanceList []Subnet
		for _, subnet := range subnetList {
			instanceList = append(instanceList, Subnet{subnet})
		}
		*object = instanceList
		return

	default:
		err = liberr.New(fmt.Sprintf("unsupported type %+v", object))
		return
	}
}

// Get a resource.
func (r *Client) get(object interface{}, ID string) (err error) {
	switch object.(type) {
	case *Region:
		var region *regions.Region
		region, err = regions.Get(r.identityService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Region{*region}
		return
	case *Project:
		var project *projects.Project
		project, err = projects.Get(r.identityService, ID).Extract()
		if err != nil {
			if !r.isForbidden(err) {
				err = liberr.Wrap(err)
				return
			}
			object, err = r.getUserProject(ID)
			if err != nil {
				err = liberr.Wrap(err)
			}
			return
		}
		object = &Project{*project}
		return
	case *Flavor:
		var flavor *flavors.Flavor
		flavor, err = flavors.Get(r.ComputeService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		var extraSpecs map[string]string
		extraSpecs, err = flavors.ListExtraSpecs(r.ComputeService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Flavor{Flavor: *flavor, ExtraSpecs: extraSpecs}

		return
	case *Image:
		var image *images.Image
		image, err = images.Get(r.ImageService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Image{*image}
		return
	case *Snapshot:
		var snapshot *snapshots.Snapshot
		snapshot, err = snapshots.Get(r.BlockStorageService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Snapshot{*snapshot}
		return
	case *Volume:
		var volume *volumes.Volume
		volume, err = volumes.Get(r.BlockStorageService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Volume{*volume}
		return
	case *VolumeType:
		var volumeType *volumetypes.VolumeType
		volumeType, err = volumetypes.Get(r.BlockStorageService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &VolumeType{*volumeType}
		return
	case *VM:
		var server *servers.Server
		server, err = servers.Get(r.ComputeService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &VM{*server}
		return
	case *Network:
		var network *networks.Network
		network, err = networks.Get(r.NetworkService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Network{*network}
		return
	case *Subnet:
		var subnet *subnets.Subnet
		subnet, err = subnets.Get(r.NetworkService, ID).Extract()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		object = &Subnet{*subnet}
		return
	default:
		err = liberr.New(fmt.Sprintf("unsupported type %+v", object))
		return
	}
}

func (r *Client) isNotFound(err error) bool {
	switch unWrapErr := liberr.Unwrap(err).(type) {
	case gophercloud.ErrUnexpectedResponseCode:
		if unWrapErr.GetStatusCode() == http.StatusNotFound {
			return true
		}
	}
	return false
}

func (r *Client) isForbidden(err error) bool {
	switch unWrapErr := liberr.Unwrap(err).(type) {
	case gophercloud.ErrUnexpectedResponseCode:
		if unWrapErr.GetStatusCode() == http.StatusForbidden {
			return true
		}
	}
	return false
}

func (r *Client) getAuthenticatedUserID() (string, error) {
	authResult := r.provider.GetAuthResult()
	if authResult == nil {
		//ProviderClient did not use openstack.Authenticate(), e.g. because token
		//was set manually with ProviderClient.SetToken()
		return "", liberr.New("no AuthResult available")
	}
	switch a := authResult.(type) {
	case tokens.CreateResult:
		u, err := a.ExtractUser()
		if err != nil {
			err = liberr.Wrap(err)
			return "", err
		}
		return u.ID, nil
	default:
		return "", liberr.New(fmt.Sprintf("got unexpected AuthResult type: %T", a))

	}
}

func (r *Client) getUserProject(projectID string) (project *Project, err error) {
	var userProjects []Project
	var found bool
	userProjects, err = r.getUserProjects()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, p := range userProjects {
		if p.ID == projectID {
			found = true
			project = &p
			break
		}
	}
	if !found {
		err = gophercloud.ErrDefault404{}
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *Client) getUserProjects() (userProjects []Project, err error) {
	var userID string
	var allPages pagination.Page
	userID, err = r.getAuthenticatedUserID()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	allPages, err = users.ListProjects(r.identityService, userID).AllPages()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var projectList []projects.Project
	projectList, err = projects.ExtractProjects(allPages)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, project := range projectList {
		// TODO implement support multiple regions/projects sync per user
		if project.Name == r.getStringFromSecret(ProjectName) {
			userProjects = append(userProjects, Project{project})
		}
	}
	return
}
