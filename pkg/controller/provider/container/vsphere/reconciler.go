package vsphere

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	liburl "net/url"
	"time"
)

//
// Settings
const (
	// Connect retry delay.
	RetryDelay = time.Second * 5
	// Max object in each update.
	MaxObjectUpdates = 10000
)

//
// Types
const (
	Folder         = "Folder"
	VirtualMachine = "VirtualMachine"
	Datacenter     = "Datacenter"
	Cluster        = "ClusterComputeResource"
	Host           = "HostSystem"
	Network        = "Network"
	DVPortGroup    = "DistributedVirtualPortgroup"
	DVSwitch       = "VmwareDistributedVirtualSwitch"
	Datastore      = "Datastore"
)

//
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
	fVm             = "vm"
	fProductName    = "config.product.name"
	fProductVersion = "config.product.version"
	fVSwitch        = "config.network.vswitch"
	fPortGroup      = "config.network.portgroup"
	fPNIC           = "config.network.pnic"
	fVNIC           = "config.network.vnic"
	fInMaintMode    = "summary.runtime.inMaintenanceMode"
	fCpuSockets     = "summary.hardware.numCpuPkgs"
	fCpuCores       = "summary.hardware.numCpuCores"
	fThumbprint     = "summary.config.sslThumbprint"
	// Network
	fTag = "tag"
	// PortGroup
	fDVSwitch = "config.distributedVirtualSwitch"
	// DV Switch
	fDVSwitchHost = "config.host"
	// Datastore
	fDsType      = "summary.type"
	fCapacity    = "summary.capacity"
	fFreeSpace   = "summary.freeSpace"
	fDsMaintMode = "summary.maintenanceMode"
	// VM
	fUUID                = "config.uuid"
	fFirmware            = "config.firmware"
	fFtInfo              = "config.ftInfo"
	fCpuAffinity         = "config.cpuAffinity"
	fCpuHotAddEnabled    = "config.cpuHotAddEnabled"
	fCpuHotRemoveEnabled = "config.cpuHotRemoveEnabled"
	fMemoryHotAddEnabled = "config.memoryHotAddEnabled"
	fNumCpu              = "config.hardware.numCPU"
	fNumCoresPerSocket   = "config.hardware.numCoresPerSocket"
	fMemorySize          = "config.hardware.memoryMB"
	fDevices             = "config.hardware.device"
	fExtraConfig         = "config.extraConfig"
	fGuestName           = "summary.config.guestFullName"
	fBalloonedMemory     = "summary.quickStats.balloonedMemory"
	fVmIpAddress         = "summary.guest.ipAddress"
	fStorageUsed         = "summary.storage.committed"
	fRuntimeHost         = "runtime.host"
	fPowerState          = "runtime.powerState"
)

//
// Selections
const (
	TraverseFolders = "traverseFolders"
)

//
// Actions
const (
	Enter  = "enter"
	Leave  = "leave"
	Modify = "modify"
	Assign = "assign"
)

//
// Folder traversal Spec.
var TsDFolder = &types.TraversalSpec{
	Type: Folder,
	Path: fChildEntity,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

//
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

//
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

//
// Datacenter/Host traversal Spec.
var TsClusterHostSystem = &types.TraversalSpec{
	Type: Cluster,
	Path: fHost,
	SelectSet: []types.BaseSelectionSpec{
		&types.SelectionSpec{
			Name: TraverseFolders,
		},
	},
}

//
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

//
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

//
// Root Folder traversal Spec
var TsRootFolder = &types.TraversalSpec{
	SelectionSpec: types.SelectionSpec{
		Name: TraverseFolders,
	},
	Type: Folder,
	Path: fChildEntity,
	SelectSet: []types.BaseSelectionSpec{
		TsDFolder,
		TsDatacenterVM,
		TsDatacenterHost,
		TsDatacenterNet,
		TsDatacenterDatastore,
		TsClusterHostSystem,
	},
}

func init() {
	logger := logging.WithName("vsphere")
	logger.Reset()
	Log = &logger
}

//
// A VMWare reconciler.
type Reconciler struct {
	// The vsphere url.
	url string
	// Provider
	provider *api.Provider
	// Credentials secret: {user:,password}.
	secret *core.Secret
	// DB client.
	db libmodel.DB
	// logger.
	log logging.Logger
	// client.
	client *govmomi.Client
	// cancel function.
	cancel func()
	// has consistency
	consistent bool
}

//
// New reconciler.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *Reconciler {
	log := logging.WithName(provider.GetName())
	return &Reconciler{
		url:      provider.Spec.URL,
		provider: provider,
		secret:   secret,
		db:       db,
		log:      log,
	}
}

//
// The name.
func (r *Reconciler) Name() string {
	url, err := liburl.Parse(r.url)
	if err == nil {
		return url.Host
	}

	return r.url
}

//
// The owner.
func (r *Reconciler) Owner() meta.Object {
	return r.provider
}

//
// Get the DB.
func (r *Reconciler) DB() libmodel.DB {
	return r.db
}

//
// Reset.
func (r *Reconciler) Reset() {
	r.consistent = false
}

//
// Reset.
func (r *Reconciler) HasConsistency() bool {
	return r.consistent
}

//
// Test connect/logout.
func (r *Reconciler) Test() (err error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.connect(ctx)
	if err == nil {
		r.client.Logout(ctx)
	}

	return
}

//
// Start the reconciler.
func (r *Reconciler) Start() error {
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
					r.log.Trace(err, "retry", RetryDelay)
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

//
// Shutdown the reconciler.
func (r *Reconciler) Shutdown() {
	r.log.Info("Shutdown.")
	if r.cancel != nil {
		r.cancel()
	}
}

//
// Get object updates.
//  1. connect.
//  2. apply updates.
// Blocks waiting on updates until canceled.
func (r *Reconciler) getUpdates(ctx context.Context) error {
	err := r.connect(ctx)
	if err != nil {
		return err
	}
	defer r.client.Logout(ctx)
	about := r.client.ServiceContent.About
	err = r.db.Insert(
		&model.About{
			APIVersion: about.ApiVersion,
			Product:    about.LicenseProductName,
		})
	if err != nil {
		return err
	}
	pc := property.DefaultCollector(r.client.Client)
	pc, err = pc.Create(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	defer pc.Destroy(context.Background())
	filter := r.filter(pc)
	filter.Options.MaxObjectUpdates = MaxObjectUpdates
	err = pc.CreateFilter(ctx, filter.CreateFilter)
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
		r.consistent = false
		for _, w := range watchList {
			r.db.EndWatch(w)
		}
		if tx != nil {
			tx.End()
		}
	}()
next:
	for {
		response, err := methods.WaitForUpdatesEx(ctx, r.client, &req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				pc.CancelWaitForUpdates(context.Background())
				break
			}
			return liberr.Wrap(err)
		}
		updateSet := response.Returnval
		if updateSet == nil {
			err := r.connect(ctx)
			if err != nil {
				r.log.Trace(err)
				time.Sleep(RetryDelay)
			}
			continue next
		}
		req.Version = updateSet.Version
		tx, err = r.db.Begin()
		if err != nil {
			return err
		}
		for _, fs := range updateSet.FilterSet {
			err = r.apply(ctx, tx, fs.ObjectSet)
			if err != nil {
				Log.Trace(err)
				break
			}
		}
		if err == nil {
			err = tx.Commit()
		} else {
			err = tx.End()
		}
		if err != nil {
			Log.Trace(err)
		}
		if updateSet.Truncated == nil || !*updateSet.Truncated {
			if !r.consistent {
				r.consistent = true
				r.log.Info("Initial consistency.", "duration", time.Since(mark))
				watchList = r.watch()
			}
		}
	}

	return nil
}

//
// Add model watches.
func (r *Reconciler) watch() (list []*libmodel.Watch) {
	// Cluster
	w, err := r.db.Watch(
		&model.Cluster{},
		&ClusterEventHandler{DB: r.db})
	if err != nil {
		Log.Trace(err)
	} else {
		list = append(list, w)
	}
	// Host
	w, err = r.db.Watch(
		&model.Host{},
		&HostEventHandler{DB: r.db})
	if err != nil {
		Log.Trace(err)
	} else {
		list = append(list, w)
	}
	// VM
	w, err = r.db.Watch(
		&model.VM{},
		&VMEventHandler{Provider: r.provider, DB: r.db})
	if err != nil {
		Log.Trace(liberr.Wrap(err))
	} else {
		list = append(list, w)
	}

	return
}

//
// Build the client.
func (r *Reconciler) connect(ctx context.Context) error {
	if r.client != nil {
		_ = r.client.Logout(ctx)
		r.client = nil
	}
	url, err := liburl.Parse(r.url)
	if err != nil {
		return liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(
		r.user(),
		r.password())
	soapClient := soap.NewClient(url, false)
	soapClient.SetThumbprint(url.Host, r.thumbprint())
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = r.client.Login(ctx, url.User)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// User.
func (r *Reconciler) user() string {
	if user, found := r.secret.Data["user"]; found {
		return string(user)
	}

	return ""
}

//
// Password.
func (r *Reconciler) password() string {
	if password, found := r.secret.Data["password"]; found {
		return string(password)
	}

	return ""
}

//
// Thumbprint.
func (r *Reconciler) thumbprint() string {
	if password, found := r.secret.Data["thumbprint"]; found {
		return string(password)
	}

	return ""
}

//
// Build the object Spec filter.
func (r *Reconciler) filter(pc *property.Collector) *property.WaitFilter {
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
		Options: &types.WaitOptions{},
	}
}

//
// Build the object Spec.
func (r *Reconciler) objectSpec() types.ObjectSpec {
	return types.ObjectSpec{
		Obj: r.client.ServiceContent.RootFolder,
		SelectSet: []types.BaseSelectionSpec{
			TsRootFolder,
		},
	}
}

//
// Build the property Spec.
func (r *Reconciler) propertySpec() []types.PropertySpec {
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
				fProductName,
				fProductVersion,
				fThumbprint,
				fInMaintMode,
				fCpuSockets,
				fCpuCores,
				fDatastore,
				fNetwork,
				fVm,
				fVSwitch,
				fPortGroup,
				fPNIC,
				fVNIC,
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
			Type: DVPortGroup,
			PathSet: []string{
				fName,
				fDVSwitch,
				fTag,
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
				fHost,
			},
		},
		{ // VM
			Type: VirtualMachine,
			PathSet: []string{
				fName,
				fParent,
				fUUID,
				fFirmware,
				fFtInfo,
				fCpuAffinity,
				fCpuHotAddEnabled,
				fCpuHotRemoveEnabled,
				fMemoryHotAddEnabled,
				fNumCpu,
				fNumCoresPerSocket,
				fMemorySize,
				fDevices,
				fExtraConfig,
				fGuestName,
				fBalloonedMemory,
				fVmIpAddress,
				fStorageUsed,
				fDatastore,
				fNetwork,
				fRuntimeHost,
				fPowerState,
			},
		},
	}
}

//
// Apply updates.
func (r *Reconciler) apply(ctx context.Context, tx *libmodel.Tx, updates []types.ObjectUpdate) (err error) {
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

//
// Select the appropriate adapter.
func (r *Reconciler) selectAdapter(u types.ObjectUpdate) (Adapter, bool) {
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
					ID: u.Obj.Value,
				},
				Variant: model.NetStandard,
			},
		}
	case DVPortGroup:
		adapter = &NetworkAdapter{
			model: model.Network{
				Base: model.Base{
					ID: u.Obj.Value,
				},
				Variant: model.NetDvPortGroup,
			},
		}
	case DVSwitch:
		adapter = &DVSwitchAdapter{
			model: model.Network{
				Base: model.Base{
					ID: u.Obj.Value,
				},
				Variant: model.NetDvSwitch,
			},
		}
	case Datastore:
		adapter = &DatastoreAdapter{
			model: model.Datastore{
				Base: model.Base{
					ID: u.Obj.Value,
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

//
// Object created.
func (r Reconciler) applyEnter(tx *libmodel.Tx, u types.ObjectUpdate) error {
	adapter, selected := r.selectAdapter(u)
	if !selected {
		return nil
	}
	adapter.Apply(u)
	m := adapter.Model()
	if mX, cast := m.(interface{ Created() }); cast {
		mX.Created()
	}
	r.log.Info("Create", "model", m.String())
	err := tx.Insert(m)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Object modified.
func (r Reconciler) applyModify(tx *libmodel.Tx, u types.ObjectUpdate) error {
	adapter, selected := r.selectAdapter(u)
	if !selected {
		return nil
	}
	m := adapter.Model()
	r.log.Info("Update", "model", m.String())
	err := tx.Get(m)
	if err != nil {
		return liberr.Wrap(err)
	}
	adapter.Apply(u)
	if mX, cast := m.(interface{ Updated() }); cast {
		mX.Updated()
	}
	err = tx.Update(m)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Object deleted.
func (r Reconciler) applyLeave(tx *libmodel.Tx, u types.ObjectUpdate) error {
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
	case Network:
		deleted = &model.Network{
			Base: model.Base{
				ID: u.Obj.Value,
			},
		}
	case Datastore:
		deleted = &model.Datastore{
			Base: model.Base{
				ID: u.Obj.Value,
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
	r.log.Info("Delete", "model", deleted.String())
	err := tx.Delete(deleted)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}
