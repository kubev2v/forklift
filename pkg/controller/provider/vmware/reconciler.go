package vmware

import (
	"context"
	"errors"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	liburl "net/url"
	"time"
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
	NetDVPG        = "DistributedVirtualPortgroup"
	Datastore      = "Datastore"
)

//
// Fields
const (
	VmFolder    = "vmFolder"
	HostFolder  = "hostFolder"
	NetFolder   = "networkFolder"
	DsFolder    = "datastoreFolder"
	ChildEntity = "childEntity"
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
	Path: ChildEntity,
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
	Path: VmFolder,
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
	Path: HostFolder,
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
	Path: "host",
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
	Path: NetFolder,
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
	Path: DsFolder,
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
	Path: ChildEntity,
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
	logger := logging.WithName("")
	logger.Reset()
	Log = &logger
}

//
// A VMWare reconciler.
type Reconciler struct {
	// The vcenter host.
	host string
	// Credentials secret: {user:,password}.
	secret *core.Secret
	// DB client.
	db libmodel.DB
	// client.
	client *govmomi.Client
	// cancel function.
	cancel func()
	// has consistency
	consistent bool
}

//
// New reconciler.
func New(host string, secret *core.Secret, db libmodel.DB) *Reconciler {
	return &Reconciler{
		host:   host,
		secret: secret,
		db:     db,
	}
}

//
// The name.
func (r *Reconciler) Name() string {
	return r.host
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
// Start the reconciler.
func (r *Reconciler) Start() error {
	err := r.db.Open(true)
	if err != nil {
		return liberr.Wrap(err)
	}
	ctx := context.Background()
	ctx, r.cancel = context.WithCancel(ctx)
	err = r.connect(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	run := func() {
		Log.Info("Started.", "name", r.Name())
		err := r.getUpdates(ctx)
		if err != nil {
			Log.Trace(err)
			return
		}
		r.client.Logout(ctx)
		Log.Info("Shutdown.", "name", r.Name())
	}

	go run()

	return nil
}

//
// Shutdown the reconciler.
func (r *Reconciler) Shutdown(purge bool) {
	r.db.Close(true)
	if r.cancel != nil {
		r.cancel()
	}
}

//
// Get updates.
func (r *Reconciler) getUpdates(ctx context.Context) error {
	pc := property.DefaultCollector(r.client.Client)
	pc, err := pc.Create(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	defer pc.Destroy(context.Background())
	filter := r.filter(pc)
	err = pc.CreateFilter(ctx, filter.CreateFilter)
	if err != nil {
		return liberr.Wrap(err)
	}
	req := types.WaitForUpdatesEx{
		This:    pc.Reference(),
		Options: filter.Options,
	}
	for {
		res, err := methods.WaitForUpdatesEx(ctx, r.client, &req)
		if err != nil {
			if ctx.Err() == context.Canceled {
				pc.CancelWaitForUpdates(context.Background())
				break
			}
			return liberr.Wrap(err)
		}
		updateSet := res.Returnval
		if updateSet == nil {
			break
		}
		req.Version = updateSet.Version
		for _, fs := range updateSet.FilterSet {
			r.apply(ctx, fs.ObjectSet)
		}
		if updateSet.Truncated == nil || !*updateSet.Truncated {
			r.consistent = true
		}
	}

	return nil
}

//
// Build the client.
func (r *Reconciler) connect(ctx context.Context) error {
	insecure := true
	url := &liburl.URL{
		Scheme: "https",
		User:   liburl.UserPassword(r.user(), r.password()),
		Host:   r.host,
		Path:   vim25.Path,
	}
	client, err := govmomi.NewClient(ctx, url, insecure)
	if err != nil {
		return liberr.Wrap(err)
	}

	r.client = client

	return nil
}

//
// User.
func (r *Reconciler) user() string {
	user := string(r.secret.Data["user"])
	return user
}

//
// Password.
func (r *Reconciler) password() string {
	password := string(r.secret.Data["password"])
	return password
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
				"name",
				"childEntity",
				"parent",
			},
		},
		{ // Datacenter
			Type: Datacenter,
			PathSet: []string{
				"name",
				VmFolder,
				HostFolder,
				NetFolder,
				DsFolder,
				"parent",
			},
		},
		{ // Cluster
			Type: Cluster,
			PathSet: []string{
				"name",
				"configuration.dasVmConfig",
				"configuration.dasConfig.enabled",
				"configuration.drsConfig",
				"host",
				"parent",
			},
		},
		{ // Host
			Type: Host,
			PathSet: []string{
				"name",
				"config.product.name",
				"config.product.version",
				"summary.runtime.inMaintenanceMode",
				"datastore",
				"network",
				"vm",
				"parent",
			},
		},
		{ // Network
			Type: Network,
			PathSet: []string{
				"name",
				"tag",
				"parent",
			},
		},
		{ // Datastore
			Type: Datastore,
			PathSet: []string{
				"name",
				"summary.type",
				"summary.capacity",
				"summary.freeSpace",
				"summary.maintenanceMode",
				"host",
				"parent",
			},
		},
		{ // VM
			Type: VirtualMachine,
			PathSet: []string{
				"name",
				"summary",
				"parent",
			},
		},
	}
}

//
// Apply updates.
func (r *Reconciler) apply(ctx context.Context, updates []types.ObjectUpdate) {
	var err error
	for _, u := range updates {
		switch string(u.Kind) {
		case Enter:
			err = r.applyEnter(ctx, u)
		case Modify:
			err = r.applyModify(ctx, u)
		case Leave:
			err = r.applyLeave(ctx, u)
		}
	}
	if err != nil {
		Log.Trace(err)
	}
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
	case Network, NetDVPG:
		adapter = &NetworkAdapter{
			model: model.Network{
				Base: model.Base{
					ID: u.Obj.Value,
				},
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
		Log.Info("Unknown", "kind", u.Obj.Type)
		return nil, false
	}

	return adapter, true
}

//
// Object created.
func (r Reconciler) applyEnter(ctx context.Context, u types.ObjectUpdate) error {
	adapter, selected := r.selectAdapter(u)
	if !selected {
		return nil
	}
	adapter.With(u)
	err := r.db.Insert(adapter.Model())
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}

//
// Object modified.
func (r Reconciler) applyModify(ctx context.Context, u types.ObjectUpdate) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		adapter, selected := r.selectAdapter(u)
		if !selected {
			return nil
		}
		err := r.db.Get(adapter.Model())
		if err != nil {
			Log.Trace(err)
			continue
		}
		adapter.With(u)
		err = r.db.Update(adapter.Model())
		if err == nil {
			break
		}
		if errors.Is(err, libmodel.Conflict) {
			Log.Info(err.Error())
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				break
			}
			continue
		} else {
			return liberr.Wrap(err)
		}
	}

	return nil
}

//
// Object deleted.
func (r Reconciler) applyLeave(ctx context.Context, u types.ObjectUpdate) error {
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
		Log.Info("Unknown", "kind", u.Obj.Type)
		return nil
	}
	err := r.db.Delete(deleted)
	if err != nil {
		return liberr.Wrap(err)
	}

	return nil
}
