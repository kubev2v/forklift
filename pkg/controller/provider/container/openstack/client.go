package openstack

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/snapshots"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/networks"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/regions"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/gophercloud/utils/openstack/clientconfig"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
)

// Client struct
type Client struct {
	URL                 string
	Secret              *core.Secret
	provider            *gophercloud.ProviderClient
	identityService     *gophercloud.ServiceClient
	computeService      *gophercloud.ServiceClient
	imageService        *gophercloud.ServiceClient
	blockStorageService *gophercloud.ServiceClient
	log                 logr.Logger
}

// Connect.
func (r *Client) Connect() (err error) {
	var TLSClientConfig *tls.Config

	if r.provider != nil {
		return
	}

	if r.insecure() {
		TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else {
		cacert := []byte(r.cacert())
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(cacert)
		if !ok {
			roots, err = x509.SystemCertPool()
			if err != nil {
				err = liberr.New("failed to configure the system's cert pool")
				return
			}
		}
		TLSClientConfig = &tls.Config{RootCAs: roots}
	}

	clientOpts := &clientconfig.ClientOpts{
		AuthInfo: &clientconfig.AuthInfo{
			AuthURL:     r.URL,
			Username:    r.username(),
			Password:    r.password(),
			ProjectName: r.projectName(),
			DomainName:  r.domainName(),
			AllowReauth: true,
		},
		HTTPClient: &http.Client{
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
		},
	}

	provider, err := clientconfig.AuthenticatedClient(clientOpts)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.provider = provider

	identityService, err := openstack.NewIdentityV3(r.provider, gophercloud.EndpointOpts{Region: r.region()})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.identityService = identityService

	computeService, err := openstack.NewComputeV2(r.provider, gophercloud.EndpointOpts{Region: r.region()})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.computeService = computeService

	imageService, err := openstack.NewImageServiceV2(r.provider, gophercloud.EndpointOpts{Region: r.region()})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.imageService = imageService

	blockStorageService, err := openstack.NewBlockStorageV3(r.provider, gophercloud.EndpointOpts{Region: r.region()})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.blockStorageService = blockStorageService

	return
}

// Username.
func (r *Client) username() string {
	if username, found := r.Secret.Data["username"]; found {
		return string(username)
	}
	return ""
}

// Password.
func (r *Client) password() string {
	if password, found := r.Secret.Data["password"]; found {
		return string(password)
	}
	return ""
}

// Project Name
func (r *Client) projectName() string {
	if projectName, found := r.Secret.Data["projectName"]; found {
		return string(projectName)
	}
	return ""
}

// Domain Name
func (r *Client) domainName() string {
	if domainName, found := r.Secret.Data["domainName"]; found {
		return string(domainName)
	}
	return ""
}

// Region
func (r *Client) region() string {
	if region, found := r.Secret.Data["region"]; found {
		return string(region)
	}
	return ""
}

// CA Certificate
func (r *Client) cacert() string {
	if cacert, found := r.Secret.Data["cacert"]; found {
		return string(cacert)
	}
	return ""
}

// Insecure
func (r *Client) insecure() bool {
	if insecure, found := r.Secret.Data["insecure"]; found {
		insecure, err := strconv.ParseBool(string(insecure))
		if err != nil {
			return false
		}
		return insecure
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

	case *[]Project:
		object := object.(*[]Project)
		allPages, err = projects.List(r.identityService, listopts.(*ProjectListOpts)).AllPages()
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

	case *[]Flavor:
		object := object.(*[]Flavor)
		allPages, err = flavors.ListDetail(r.computeService, listopts.(*FlavorListOpts)).AllPages()
		if err != nil {
			return
		}
		var flavorList []flavors.Flavor
		flavorList, err = flavors.ExtractFlavors(allPages)
		if err != nil {
			return
		}
		var instanceList []Flavor
		for _, flavor := range flavorList {
			instanceList = append(instanceList, Flavor{flavor})
		}
		*object = instanceList
		return

	case *[]Image:
		object := object.(*[]Image)
		allPages, err = images.List(r.imageService, listopts.(*ImageListOpts)).AllPages()
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

	case *[]VM:
		object := object.(*[]VM)
		allPages, err = servers.List(r.computeService, listopts.(*VMListOpts)).AllPages()
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

	case *[]Snapshot:
		object := object.(*[]Snapshot)
		allPages, err = snapshots.List(r.blockStorageService, nil).AllPages()
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

	case *[]Volume:
		object := object.(*[]Volume)
		allPages, err = volumes.List(r.blockStorageService, listopts.(*VolumeListOpts)).AllPages()
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

	case *[]VolumeType:
		object := object.(*[]VolumeType)
		allPages, err = volumetypes.List(r.blockStorageService, listopts.(*VolumeTypeListOpts)).AllPages()
		if err != nil {
			return
		}
		var volumeTypeList []volumetypes.VolumeType
		volumeTypeList, err = volumetypes.ExtractVolumeTypes(allPages)
		if err != nil {
			return
		}
		var instanceList []VolumeType
		for _, volume := range volumeTypeList {
			instanceList = append(instanceList, VolumeType{volume})
		}
		*object = instanceList
		return

	case *[]Network:
		object := object.(*[]Network)
		allPages, err = networks.List(r.computeService).AllPages()
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
			return
		}
		object = &Region{*region}
		return
	case *Project:
		var project *projects.Project
		project, err = projects.Get(r.identityService, ID).Extract()
		if err != nil {
			return
		}
		object = &Project{*project}
		return
	case *Flavor:
		var flavor *flavors.Flavor
		flavor, err = flavors.Get(r.computeService, ID).Extract()
		if err != nil {
			return
		}
		object = &Flavor{*flavor}
		return
	case *Image:
		var image *images.Image
		image, err = images.Get(r.imageService, ID).Extract()
		if err != nil {
			return
		}
		object = &Image{*image}
		return
	case *Snapshot:
		var snapshot *snapshots.Snapshot
		snapshot, err = snapshots.Get(r.blockStorageService, ID).Extract()
		if err != nil {
			return
		}
		object = &Snapshot{*snapshot}
		return
	case *Volume:
		var volume *volumes.Volume
		volume, err = volumes.Get(r.blockStorageService, ID).Extract()
		if err != nil {
			return
		}
		object = &Volume{*volume}
		return
	case *VolumeType:
		var volumeType *volumetypes.VolumeType
		volumeType, err = volumetypes.Get(r.blockStorageService, ID).Extract()
		if err != nil {
			return
		}
		object = &VolumeType{*volumeType}
		return
	case *VM:
		var server *servers.Server
		server, err = servers.Get(r.computeService, ID).Extract()
		if err != nil {
			return
		}
		object = &VM{*server}
		return
	case *Network:
		var network *networks.Network
		network, err = networks.Get(r.computeService, ID).Extract()
		if err != nil {
			return
		}
		object = &Network{*network}
		return
	default:
		err = liberr.New(fmt.Sprintf("unsupported type %+v", object))
		return
	}
}

func (r *Client) isNotFound(err error) bool {
	switch liberr.Unwrap(err).(type) {
	case gophercloud.ErrResourceNotFound, gophercloud.ErrDefault404:
		return true
	default:
		return false
	}
}
