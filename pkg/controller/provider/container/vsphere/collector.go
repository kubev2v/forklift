package vsphere

import (
	"context"
	"fmt"
	"net/http"
	liburl "net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/kubev2v/forklift/pkg/lib/util"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Settings
const (
	// Connect retry delay.
	RetryDelay = time.Second * 5
	// Max object in each update.
	MaxObjectUpdates = 10000
	// Connection timeout for provider operations.
	ConnectionTimeout = 30 * time.Second
)

// Types
const (
	Folder          = "Folder"
	VirtualMachine  = "VirtualMachine"
	Datacenter      = "Datacenter"
	Cluster         = "ClusterComputeResource"
	ComputeResource = "ComputeResource"
	Host            = "HostSystem"
	Network         = "Network"
	OpaqueNetwork   = "OpaqueNetwork"
	DVPortGroup     = "DistributedVirtualPortgroup"
	DVSwitch        = "VmwareDistributedVirtualSwitch"
	Datastore       = "Datastore"
	ResourcePool    = "ResourcePool"
)

// Fields
const (
	// Common
	fName      = "name"
	fParent    = "parent"
	fHost      = "host"
	fNetwork   = "network"
	fDatastore = "datastore"
	// Folders
	fVmFolder    = "vmFolder"
	fHostFolder  = "hostFolder"
	fNetFolder   = "networkFolder"
	fDsFolder    = "datastoreFolder"
	fChildEntity = "childEntity"
	// Cluster
	fDasEnabled    = "configuration.dasConfig.enabled"
	fDasVmCfg      = "configuration.dasVmConfig"
	fDrsEnabled    = "configuration.drsConfig.enabled"
	fDrsVmBehavior = "configuration.drsConfig.defaultVmBehavior"
	fDrsVmCfg      = "configuration.drsVmConfig"
	// Host
	fVm                   = "vm"
	fOverallStatus        = "overallStatus"
	fProductName          = "config.product.name"
	fProductVersion       = "config.product.version"
	fVSwitch              = "config.network.vswitch"
	fPortGroup            = "config.network.portgroup"
	fPNIC                 = "config.network.pnic"
	fVNIC                 = "config.network.vnic"
	fVirtualNicManagerNet = "config.virtualNicManagerInfo.netConfig"
	fTimezone             = "config.dateTimeInfo.timeZone.name"
	fInMaintMode          = "summary.runtime.inMaintenanceMode"
	fCpuSockets           = "summary.hardware.numCpuPkgs"
	fCpuCores             = "summary.hardware.numCpuCores"
	fHostMemorySize       = "summary.hardware.memorySize"
	fThumbprint           = "summary.config.sslThumbprint"
	fMgtServerIp          = "summary.managementServerIp"
	fScsiLun              = "config.storageDevice.scsiLun"
	fHostBusAdapter       = "config.storageDevice.hostBusAdapter"
	fScsiTopology         = "config.storageDevice.scsiTopology.adapter"
	fAdvancedOption       = "configManager.advancedOption"
	fmodel                = "hardware.systemInfo.model"
	fvendor               = "hardware.systemInfo.vendor"
	// Network
	fTag     = "tag"
	fSummary = "summary"
	// PortGroup
	fDVSwitch     = "config.distributedVirtualSwitch"
	fDVSwitchVlan = "config.defaultPortConfig"
	fKey          = "key"
	// DV Switch
	fDVSwitchHost = "config.host"
	// ResourcePool
	fResourcePool = "resourcePool"
	// Datastore
	fDsType      = "summary.type"
	fCapacity    = "summary.capacity"
	fFreeSpace   = "summary.freeSpace"
	fDsMaintMode = "summary.maintenanceMode"
	fVmfsExtent  = "info"
	// VM
	fUUID                     = "config.uuid"
	fFirmware                 = "config.firmware"
	fFtInfo                   = "config.ftInfo"
	fBootOptions              = "config.bootOptions"
	fCpuAffinity              = "config.cpuAffinity"
	fCpuHotAddEnabled         = "config.cpuHotAddEnabled"
	fCpuHotRemoveEnabled      = "config.cpuHotRemoveEnabled"
	fMemoryHotAddEnabled      = "config.memoryHotAddEnabled"
	fNumCpu                   = "config.hardware.numCPU"
	fNumCoresPerSocket        = "config.hardware.numCoresPerSocket"
	fMemorySize               = "config.hardware.memoryMB"
	fDevices                  = "config.hardware.device"
	fExtraConfig              = "config.extraConfig"
	fNestedHVEnabled          = "config.nestedHVEnabled"
	fChangeTracking           = "config.changeTrackingEnabled"
	fGuestName                = "summary.config.guestFullName"
	fGuestNameFromVmwareTools = "guest.guestFullName"
	fGuestID                  = "summary.guest.guestId"
	fTpmPresent               = "summary.config.tpmPresent"
	fBalloonedMemory          = "summary.quickStats.balloonedMemory"
	fVmIpAddress              = "summary.guest.ipAddress"
	fStorageUsed              = "summary.storage.committed"
	fRuntimeHost              = "runtime.host"
	fPowerState               = "runtime.powerState"
	fConnectionState          = "runtime.connectionState"
	fSnapshot                 = "snapshot"
	fIsTemplate               = "config.template"
	fGuestNet                 = "guest.net"
	fGuestDisk                = "guest.disk"
	fGuestIpStack             = "guest.ipStack"
	fHostName                 = "guest.hostName"
	// fToolsStatus is deprecated since vSphere API 4.0; use fToolsRunningStatus instead
	fToolsStatus        = "guest.toolsStatus"
	fToolsRunningStatus = "guest.toolsRunningStatus"
	// fToolsVersionStatus is deprecated since vSphere API 5.1; use fToolsVersionStatus2 for more detailed status
	fToolsVersionStatus = "guest.toolsVersionStatus2"
)

// Selections
const (
	TraverseFolders = "traverseFolders"
	TraverseVApps   = "TraverseVApps"
)

// Actions
const (
	Enter  = "enter"
	Leave  = "leave"
	Modify = "modify"
	Assign = "assign"
)

// Datacenter/VM traversal Spec.
var TsDatacenterVM = &types.TraversalSpec{
	Type: Datacenter,
	Path: fVmFolder,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// Datacenter/Host traversal Spec.
var TsDatacenterHost = &types.TraversalSpec{
	Type: Datacenter,
	Path: fHostFolder,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// ComputeResource/Host traversal Spec.
var TsComputeResourceHost = &types.TraversalSpec{
	Type: ComputeResource,
	Path: fHost,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// Datacenter/Host traversal Spec.
var TsDatacenterNet = &types.TraversalSpec{
	Type: Datacenter,
	Path: fNetFolder,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// Datacenter/Datastore traversal Spec.
var TsDatacenterDatastore = &types.TraversalSpec{
	Type: Datacenter,
	Path: fDsFolder,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// Datacenter/nested VApp traversal Spec.
var TsDatacenterNestedVApp = &types.TraversalSpec{
	SelectionSpec: types.SelectionSpec{
		Name: TraverseVApps, // Unique name for this TraversalSpec
	},
	Type: ResourcePool,  // vApps are ResourcePools in vSphere
	Path: fResourcePool, // Traverse nested resource pools
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseVApps, // Recursively traverse nested vApps
		},
		TsDatacenterVApp, // Traverse VMs in the vApp
	},
}

// Datacenter/root VApp traversal Spec.
var TsDatacenterVApp = &types.TraversalSpec{
	Type: ResourcePool,
	Path: fVm,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

// Root Folder traversal Spec
var TsRootFolder = &types.TraversalSpec{
	SelectionSpec: types.SelectionSpec{
		Name: TraverseFolders,
	},
	Type: Folder,
	Path: fChildEntity,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
		TsComputeResourceHost,
		TsDatacenterVM,
		TsDatacenterHost,
		TsDatacenterNet,
		TsDatacenterDatastore,
		TsDatacenterVApp,
		TsDatacenterNestedVApp,
	},
}

// A VMWare collector.
type Collector struct {
	// The vsphere url.
	url string
	// Provider
	provider *api.Provider
	// Credentials secret: {user:,password}.
	secret *core.Secret
	// DB client.
	db libmodel.DB
	// logger.
	log logging.LevelLogger
	// client.
	client *govmomi.Client
	// cancel function.
	cancel func()
	// has parity.
	parity bool
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *Collector {
	nlog := logging.WithName("collector|vsphere").WithValues(
		"provider",
		path.Join(
			provider.GetNamespace(),
			provider.GetName()))
	return &Collector{
		url:      provider.Spec.URL,
		provider: provider,
		secret:   secret,
		db:       db,
		log:      nlog,
	}
}

// The name.
func (r *Collector) Name() string {
	url, err := liburl.Parse(r.url)
	if err == nil {
		return url.Host
	}

	return r.url
}

// The owner.
func (r *Collector) Owner() meta.Object {
	return r.provider
}

// Get the DB.
func (r *Collector) DB() libmodel.DB {
	return r.db
}

// Reset.
func (r *Collector) Reset() {
	r.parity = false
}

// Reset.
func (r *Collector) HasParity() bool {
	return r.parity
}

// Follow
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	ref, ok := moRef.(types.ManagedObjectReference)
	if !ok {
		return fmt.Errorf("reference must be of type ManagedObjectReference")
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, ConnectionTimeout)
	defer cancel()
	client, err := r.buildClient(ctx)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	return client.RetrieveOne(ctx, ref, p, dst)
}

// Test connect/logout.
func (r *Collector) Test() (status int, err error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, ConnectionTimeout)
	defer cancel()
	status, err = r.connect(ctx)
	if err == nil {
		r.close()
	}

	return
}

// NO-OP
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)
	start := func() {
	try:
		for {
			select {
			case <-ctx.Done():
				break try
			default:
				err := r.getUpdates(ctx)
				if err != nil {
					r.log.Error(
						err,
						"start failed.",
						"retry",
						RetryDelay)
					time.Sleep(RetryDelay)
					continue try
				}
				break try
			}
		}
	}

	go start()

	return nil
}

// Shutdown the collector.
func (r *Collector) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

// Get object updates.
//  1. connect.
//  2. apply updates.
//
// Blocks waiting on updates until canceled.
func (r *Collector) getUpdates(ctx context.Context) error {
	_, err := r.connect(ctx)
	if err != nil {
		return err
	}
	defer r.close()
	about := r.client.ServiceContent.About
	err = r.db.Insert(
		&model.About{
			APIVersion:   about.ApiVersion,
			Product:      about.LicenseProductName,
			InstanceUuid: about.InstanceUuid,
		})
	if err != nil {
		return err
	}
	pc := property.DefaultCollector(r.client.Client)
	pc, err = pc.Create(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	defer func() {
		err := pc.Destroy(context.Background())
		if err != nil {
			r.log.Error(err, "destroy failed.")
		}
	}()

	filter := r.filter(pc)
	_, err = pc.CreateFilter(ctx, filter.CreateFilter)
	if err != nil {
		return liberr.Wrap(err)
	}
	mark := time.Now()
	req := types.WaitForUpdatesEx{
		This:    pc.Reference(),
		Options: filter.Options,
	}
	var tx *libmodel.Tx
	watchList := []*libmodel.Watch{}
	defer func() {
		r.parity = false
		for _, w := range watchList {
			w.End()
		}
		if tx != nil {
			err := tx.End()
			if err != nil {
				r.log.Error(err, "tx end failed.")
			}
		}
	}()
	for {
		response, err := methods.WaitForUpdatesEx(ctx, r.client, &req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				err = pc.CancelWaitForUpdates(context.Background())
				if err != nil {
					r.log.Error(
						err,
						"cancel wait for updates failed.")
				}

				break
			}
			return liberr.Wrap(err)
		}
		updateSet := response.Returnval
		if updateSet == nil {
			continue
		}
		req.Version = updateSet.Version
		tx, err = r.db.Begin()
		if err != nil {
			return err
		}
		for _, fs := range updateSet.FilterSet {
			err = r.apply(ctx, tx, fs.ObjectSet)
			if err != nil {
				r.log.Error(
					err,
					"apply changes failed.")
				break
			}
		}
		if err == nil {
			err = tx.Commit()
		} else {
			err = tx.End()
		}
		if err != nil {
			r.log.Error(
				err,
				"tx commit failed.")
		}
		if updateSet.Truncated == nil || !*updateSet.Truncated {
			if !r.parity {
				r.parity = true
				r.log.Info(
					"Initial parity.",
					"duration",
					time.Since(mark))
				watchList = r.watch()
			}
		}
	}

	return nil
}

// Add model watches.
func (r *Collector) watch() (list []*libmodel.Watch) {
	// Cluster
	w, err := r.db.Watch(
		&model.Cluster{},
		&ClusterEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (cluster) watch failed.")
	} else {
		list = append(list, w)
	}
	// Host
	w, err = r.db.Watch(
		&model.Host{},
		&HostEventHandler{
			DB:  r.db,
			log: r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (host) watch failed.")
	} else {
		list = append(list, w)
	}
	// VM
	w, err = r.db.Watch(
		&model.VM{},
		&VMEventHandler{
			Provider: r.provider,
			DB:       r.db,
			log:      r.log,
		})
	if err != nil {
		r.log.Error(
			err,
			"create (VM) watch failed.")
	} else {
		list = append(list, w)
	}

	return
}

// Build the client.
func (r *Collector) connect(ctx context.Context) (status int, err error) {
	r.close()
	r.client, err = r.buildClient(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "incorrect") && strings.Contains(err.Error(), "password") {
			return http.StatusUnauthorized, err
		}
		return
	}

	if err = r.validateServerType(); err != nil {
		r.close()
		return
	}
	return http.StatusOK, nil
}

func (r *Collector) validateServerType() error {
	sdkEndpoint := r.provider.Spec.Settings[api.SDK]
	isVC := r.client.IsVC()
	if sdkEndpoint == api.VCenter && !isVC {
		return liberr.New("provider sdkEndpoint is set to vCenter but the URL points to an ESXi host")
	}
	if sdkEndpoint == api.ESXI && isVC {
		return liberr.New("provider sdkEndpoint is set to ESXi but the URL points to a vCenter server")
	}
	return nil
}

// Build the client.
func (r *Collector) buildClient(ctx context.Context) (*govmomi.Client, error) {
	url, err := liburl.Parse(r.url)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(
		r.user(),
		r.password())
	thumbprint := r.thumbprint()
	skipVerifying := base.GetInsecureSkipVerifyFlag(r.secret)

	if !skipVerifying {
		cert, errtls := base.VerifyTLSConnection(r.url, r.secret)
		if errtls != nil {
			return nil, liberr.Wrap(errtls)
		}
		thumbprint = util.Fingerprint(cert)
	}

	soapClient := soap.NewClient(url, skipVerifying)
	soapClient.SetThumbprint(url.Host, thumbprint)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	client := &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = client.Login(ctx, url.User)
	return client, err

}

// Close connections.
func (r *Collector) close() {
	if r.client != nil {
		_ = r.client.Logout(context.TODO())
		r.client.CloseIdleConnections()
		r.client = nil
	}
}

// User.
func (r *Collector) user() string {
	if user, found := r.secret.Data["user"]; found {
		return string(user)
	}

	return ""
}

// Password.
func (r *Collector) password() string {
	if password, found := r.secret.Data["password"]; found {
		return string(password)
	}

	return ""
}

// Thumbprint.
func (r *Collector) thumbprint() string {
	return r.provider.Status.Fingerprint
}

// Build the object Spec filter.
func (r *Collector) filter(pc *property.Collector) *property.WaitFilter {
	return &property.WaitFilter{
		CreateFilter: types.CreateFilter{
			This: pc.Reference(),
			Spec: types.PropertyFilterSpec{
				ObjectSet: []types.ObjectSpec{
					r.objectSpec(),
				},
				PropSet: r.propertySpec(),
			},
		},
		WaitOptions: property.WaitOptions{Options: &types.WaitOptions{
			MaxObjectUpdates: MaxObjectUpdates}},
	}
}

// Build the object Spec.
func (r *Collector) objectSpec() types.ObjectSpec {
	return types.ObjectSpec{
		Obj: r.client.ServiceContent.RootFolder,
		SelectSet: []types.BaseSelectionSpec{
			TsRootFolder,
		},
	}
}

// Build the property Spec.
func (r *Collector) propertySpec() []types.PropertySpec {
	return []types.PropertySpec{
		{ // Folder
			Type: Folder,
			PathSet: []string{
				fName,
				fParent,
				fChildEntity,
			},
		},
		{ // Datacenter
			Type: Datacenter,
			PathSet: []string{
				fName,
				fParent,
				fVmFolder,
				fHostFolder,
				fNetFolder,
				fDsFolder,
			},
		},
		{ // ComputeResource
			Type: ComputeResource,
			PathSet: []string{
				fName,
				fParent,
				fHost,
			},
		},
		{ // Cluster
			Type: Cluster,
			PathSet: []string{
				fName,
				fParent,
				fDasEnabled,
				fDasVmCfg,
				fDrsEnabled,
				fDrsVmBehavior,
				fDrsVmCfg,
				fHost,
				fNetwork,
				fDatastore,
			},
		},
		{ // Host
			Type: Host,
			PathSet: []string{
				fName,
				fParent,
				fOverallStatus,
				fProductName,
				fProductVersion,
				fThumbprint,
				fTimezone,
				fMgtServerIp,
				fInMaintMode,
				fCpuSockets,
				fCpuCores,
				fHostMemorySize,
				fDatastore,
				fNetwork,
				fVSwitch,
				fPortGroup,
				fPNIC,
				fVNIC,
				fVirtualNicManagerNet,
				fScsiLun,
				fAdvancedOption,
				fHostBusAdapter,
				fScsiTopology,
				fmodel,
				fvendor,
			},
		},
		{ // Network
			Type: Network,
			PathSet: []string{
				fName,
				fParent,
				fTag,
			},
		},
		{
			Type: OpaqueNetwork,
			PathSet: []string{
				fName,
				fParent,
				fTag,
				fSummary,
			},
		},
		{
			Type: DVPortGroup,
			PathSet: []string{
				fName,
				fDVSwitch,
				fDVSwitchVlan,
				fTag,
				fKey,
			},
		},
		{
			Type: DVSwitch,
			PathSet: []string{
				fName,
				fParent,
				fDVSwitchHost,
			},
		},
		{ // Datastore
			Type: Datastore,
			PathSet: []string{
				fName,
				fParent,
				fDsType,
				fCapacity,
				fFreeSpace,
				fDsMaintMode,
				fVmfsExtent,
				fHost,
			},
		},
		{ // VM
			Type:    VirtualMachine,
			PathSet: r.vmPathSet(),
		},
	}
}

func (r *Collector) vmPathSet() []string {
	pathSet := []string{
		fName,
		fParent,
		fUUID,
		fFirmware,
		fFtInfo,
		fCpuAffinity,
		fBootOptions,
		fCpuHotAddEnabled,
		fCpuHotRemoveEnabled,
		fMemoryHotAddEnabled,
		fNumCpu,
		fNumCoresPerSocket,
		fMemorySize,
		fDevices,
		fGuestNet,
		fGuestDisk,
		fExtraConfig,
		fNestedHVEnabled,
		fGuestName,
		fGuestNameFromVmwareTools,
		fGuestID,
		fBalloonedMemory,
		fVmIpAddress,
		fStorageUsed,
		fDatastore,
		fNetwork,
		fRuntimeHost,
		fPowerState,
		fConnectionState,
		fIsTemplate,
		fSnapshot,
		fChangeTracking,
		fGuestIpStack,
		fHostName,
		fToolsStatus,
		fToolsRunningStatus,
		fToolsVersionStatus,
	}

	apiVer := strings.Split(r.client.ServiceContent.About.ApiVersion, ".")
	majorVal, _ := strconv.Atoi(apiVer[0])
	minorVal, _ := strconv.Atoi(apiVer[1])
	if majorVal > 6 || majorVal == 6 && minorVal >= 7 {
		pathSet = append(pathSet, fTpmPresent)
	}
	return pathSet
}

// Apply updates.
func (r *Collector) apply(ctx context.Context, tx *libmodel.Tx, updates []types.ObjectUpdate) (err error) {
	for _, u := range updates {
		switch string(u.Kind) {
		case Enter:
			err = r.applyEnter(tx, u)
		case Modify:
			err = r.applyModify(tx, u)
		case Leave:
			err = r.applyLeave(tx, u)
		}
		if err != nil {
			err = liberr.Wrap(err)
			break
		}
	}

	return
}

// Select the appropriate adapter.
func (r *Collector) selectAdapter(u types.ObjectUpdate) (Adapter, bool) {
	var adapter Adapter
	switch u.Obj.Type {
	case Folder:
		adapter = &FolderAdapter{
			model: model.Folder{
				Base: model.Base{
					ID: u.Obj.Value,
				},
			},
		}
	case Datacenter:
		adapter = &DatacenterAdapter{
			model: model.Datacenter{
				Base: model.Base{
					ID: u.Obj.Value,
				},
			},
		}
	case ComputeResource:
		adapter = &ClusterAdapter{
			model: model.Cluster{
				Base: model.Base{
					Variant: model.ComputeResource,
					ID:      u.Obj.Value,
				},
			},
		}
	case Cluster:
		adapter = &ClusterAdapter{
			model: model.Cluster{
				Base: model.Base{
					ID: u.Obj.Value,
				},
			},
		}
	case Host:
		adapter = &HostAdapter{
			model: model.Host{
				Base: model.Base{
					ID: u.Obj.Value,
				},
			},
		}
	case Network:
		adapter = &NetworkAdapter{
			model: model.Network{
				Base: model.Base{
					Variant: model.NetStandard,
					ID:      u.Obj.Value,
				},
			},
		}
	case OpaqueNetwork:
		adapter = &NetworkAdapter{
			model: model.Network{
				Base: model.Base{
					Variant: model.OpaqueNetwork,
					ID:      u.Obj.Value,
				},
			},
		}
	case DVPortGroup:
		adapter = &NetworkAdapter{
			model: model.Network{
				Base: model.Base{
					Variant: model.NetDvPortGroup,
					ID:      u.Obj.Value,
				},
			},
		}
	case DVSwitch:
		adapter = &DVSwitchAdapter{
			model: model.Network{
				Base: model.Base{
					Variant: model.NetDvSwitch,
					ID:      u.Obj.Value,
				},
			},
		}
	case Datastore:
		// when we get datastores from the ESXi SDK, their identifier may be in the
		// form of '10.11.12.13:/vol/virtv2v/function' which leads to invalid
		// endpoints in the inventory so we sanitize such identifiers
		datastoreId, changed := sanitize(u.Obj.Value)
		if changed {
			r.log.Info("sanitized datastore ID", "reported", u.Obj.Value, "sanitized", datastoreId)
		}
		adapter = &DatastoreAdapter{
			model: model.Datastore{
				Base: model.Base{
					ID: datastoreId,
				},
			},
		}
	case VirtualMachine:
		adapter = &VmAdapter{
			model: model.VM{
				Base: model.Base{
					ID: u.Obj.Value,
				},
			},
		}
	default:
		r.log.Info("Unknown", "kind", u.Obj.Type)
		return nil, false
	}

	return adapter, true
}

// Object created.
func (r Collector) applyEnter(tx *libmodel.Tx, u types.ObjectUpdate) error {
	adapter, selected := r.selectAdapter(u)
	if !selected {
		return nil
	}
	adapter.Apply(u)
	m := adapter.Model()
	err := tx.Insert(m)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Object modified.
func (r Collector) applyModify(tx *libmodel.Tx, u types.ObjectUpdate) error {
	adapter, selected := r.selectAdapter(u)
	if !selected {
		return nil
	}
	m := adapter.Model()
	err := tx.Get(m)
	if err != nil {
		return liberr.Wrap(err)
	}
	adapter.Apply(u)
	err = tx.Update(m)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

// Object deleted.
func (r Collector) applyLeave(tx *libmodel.Tx, u types.ObjectUpdate) error {
	var deleted model.Model
	switch u.Obj.Type {
	case Folder:
		deleted = &model.Folder{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Datacenter:
		deleted = &model.Datacenter{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Cluster:
		deleted = &model.Cluster{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Host:
		deleted = &model.Host{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Network, OpaqueNetwork, DVPortGroup, DVSwitch:
		deleted = &model.Network{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Datastore:
		datastoreId, _ := sanitize(u.Obj.Value)
		deleted = &model.Datastore{
			Base: model.Base{
				ID: datastoreId,
			},
		}
	case VirtualMachine:
		deleted = &model.VM{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	default:
		r.log.Info("Unknown", "kind", u.Obj.Type)
		return nil
	}
	err := tx.Delete(deleted)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}
