package hyperv

import (
	"context"
	"fmt"
	liburl "net/url"
	"os"
	libpath "path"
	"sort"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Settings
const (
	// Retry interval.
	RetryInterval = 5 * time.Second
	// Default refresh interval.
	DefaultRefreshInterval = 10 * time.Second
	// Env var to override refresh interval.
	EnvRefreshInterval = "HYPERV_REFRESH_INTERVAL"
)

var RefreshInterval = DefaultRefreshInterval

func init() {
	if s := os.Getenv(EnvRefreshInterval); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			RefreshInterval = d
		}
	}
}

// Phases
const (
	Started = ""
	Load    = "load"
	Loaded  = "loaded"
	Parity  = "parity"
	Refresh = "refresh"
)

// SortNICsByGuestNetworkOrder reorders vm.NICs to match the MAC address order of vm.GuestNetworks.
func SortNICsByGuestNetworkOrder(vm *model.VM) {
	macToDeviceIndex := make(map[string]int)
	for _, gn := range vm.GuestNetworks {
		if _, exists := macToDeviceIndex[gn.MAC]; !exists {
			macToDeviceIndex[gn.MAC] = gn.DeviceIndex
		}
	}

	sort.SliceStable(vm.NICs, func(i, j int) bool {
		iIdx, iOk := macToDeviceIndex[vm.NICs[i].MAC]
		jIdx, jOk := macToDeviceIndex[vm.NICs[j].MAC]

		switch {
		case iOk && jOk:
			return iIdx < jIdx
		case iOk:
			return true
		case jOk:
			return false
		default:
			return vm.NICs[i].DeviceIndex < vm.NICs[j].DeviceIndex
		}
	})
}

// HyperV data collector.
type Collector struct {
	// Provider
	provider *api.Provider
	// Provider secret
	secret *core.Secret
	// DB client.
	db libmodel.DB
	// Logger.
	log logging.LevelLogger
	// has parity.
	parity bool
	// REST client.
	client *Client
	// cancel function.
	cancel func()
	// Start time.
	startTime time.Time
	// Phase
	phase string
	// List of watches.
	watches []*libmodel.Watch
}

// New collector.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *Collector {
	log := logging.WithName("collector|hyperv").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))
	clientLog := logging.WithName("client|hyperv").WithValues(
		"provider",
		libpath.Join(
			provider.GetNamespace(),
			provider.GetName()))

	return &Collector{
		client: &Client{
			Secret: secret,
			Log:    clientLog,
		},
		provider: provider,
		secret:   secret,
		db:       db,
		log:      log,
	}
}

// The name.
func (r *Collector) Name() string {
	url, err := liburl.Parse(r.provider.Spec.URL)
	if err == nil {
		return url.Host
	}
	return r.provider.Spec.URL
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

// Test connect/logout.
func (r *Collector) Test() (_ int, err error) {
	err = r.client.Connect(r.provider)
	return
}

// NO-OP
func (r *Collector) Version() (_, _, _, _ string, err error) {
	return
}

// Follow link
func (r *Collector) Follow(moRef interface{}, p []string, dst interface{}) error {
	return fmt.Errorf("not implemented")
}

// Start the collector.
func (r *Collector) Start() error {
	ctx := Context{
		client: r.client,
		db:     r.db,
		log:    r.log,
	}
	ctx.ctx, r.cancel = context.WithCancel(context.Background())
	start := func() {
		defer func() {
			r.endWatch()
			r.log.Info("Stopped.")
		}()
		for {
			if !ctx.canceled() {
				_ = r.run(&ctx)
			} else {
				return
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

// Run the current phase.
func (r *Collector) run(ctx *Context) (err error) {
	r.log.V(1).Info("Run started.")
	r.startTime = time.Now()
	r.phase = Started

	defer func() {
		if err != nil {
			r.log.Error(err, "Run failed.")
		}
	}()

	// Connect to provider server
	err = r.client.Connect(r.provider)
	if err != nil {
		return
	}

	// Perform initial load
	err = r.load(ctx)
	if err != nil {
		return
	}

	r.parity = true
	r.phase = Parity
	r.log.Info("Initial inventory loaded.",
		"vms", r.vmCount(),
		"networks", r.networkCount(),
		"storages", r.storageCount(),
		"disks", r.diskCount())

	// Start periodic refresh
	r.beginWatch()
	for {
		select {
		case <-time.After(RefreshInterval):
			err = r.refresh(ctx)
			if err != nil {
				r.log.Error(err, "Refresh failed.")
			}
		case <-ctx.ctx.Done():
			return nil
		}
	}
}

// Load the inventory.
func (r *Collector) load(ctx *Context) (err error) {
	r.phase = Load
	mark := time.Now()
	for _, adapter := range adapterList {
		if ctx.canceled() {
			return
		}
		err = r.create(ctx, adapter)
		if err != nil {
			return
		}
	}
	r.phase = Loaded
	r.log.Info(
		"Initial Parity.",
		"duration",
		time.Since(mark))
	return
}

// List and create resources using the adapter.
func (r *Collector) create(ctx *Context, adapter Adapter) (err error) {
	itr, aErr := adapter.List(ctx, r.provider)
	if aErr != nil {
		err = aErr
		return
	}
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for {
		object, hasNext := itr.Next()
		if !hasNext {
			break
		}
		if ctx.canceled() {
			return
		}
		m := object.(libmodel.Model)
		err = tx.Insert(m)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

// Refresh the inventory.
//   - List modified vms.
//   - Build the changeSet.
//   - Apply the changeSet.
//
// The two-phased approach ensures we do not hold the
// DB transaction while using the provider API which
// can block or be slow.
func (r *Collector) refresh(ctx *Context) (err error) {
	r.phase = Refresh
	var deletions, updates []Updater
	mark := time.Now()
	for _, adapter := range adapterList {
		if ctx.canceled() {
			return
		}
		deletions, err = adapter.DeleteUnexisting(ctx)
		if err != nil {
			return
		}
		err = r.apply(deletions)
		if err != nil {
			return
		}
		updates, err = adapter.GetUpdates(ctx)
		if err != nil {
			return
		}
		err = r.apply(updates)
		if err != nil {
			return
		}
	}
	r.log.Info(
		"Refresh finished.",
		"duration",
		time.Since(mark))
	return
}

// Apply the changeSet.
func (r *Collector) apply(changeSet []Updater) (err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return
	}
	defer func() {
		_ = tx.End()
	}()
	for _, updater := range changeSet {
		err = updater(tx)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

// vmCount returns the number of VMs in the database.
func (r *Collector) vmCount() int {
	count, _ := r.db.Count(&model.VM{}, nil)
	return int(count)
}

// networkCount returns the number of networks in the database.
func (r *Collector) networkCount() int {
	count, _ := r.db.Count(&model.Network{}, nil)
	return int(count)
}

// storageCount returns the number of storages in the database.
func (r *Collector) storageCount() int {
	count, _ := r.db.Count(&model.Storage{}, nil)
	return int(count)
}

// diskCount returns the number of disks in the database.
func (r *Collector) diskCount() int {
	count, _ := r.db.Count(&model.Disk{}, nil)
	return int(count)
}

// Add model watches.
func (r *Collector) beginWatch() {
	w, err := r.db.Watch(
		&model.VM{},
		&VMEventHandler{
			Provider: r.provider,
			DB:       r.db,
			log:      r.log,
		})
	if err != nil {
		r.log.Error(err, "VM watch failed.")
		return
	}
	r.watches = append(r.watches, w)
}

// End watches.
func (r *Collector) endWatch() {
	for _, w := range r.watches {
		w.End()
	}
	r.watches = nil
}

// HyperVCredentials returns the HyperV/WinRM credentials from the secret.
func (r *Collector) HyperVCredentials() (username, password string) {
	return hvutil.HyperVCredentials(r.secret)
}

// SMBCredentials returns the SMB credentials from the secret.
func (r *Collector) SMBCredentials() (username, password string) {
	return hvutil.SMBCredentials(r.secret)
}

// SMBPath returns the local mount point where SMB is mounted in the pod.
func (r *Collector) SMBPath() string {
	return hvutil.SMBMountPath
}

// SMBUrl returns the SMB share URL from the secret.
func (r *Collector) SMBUrl() string {
	return hvutil.SMBUrl(r.secret)
}

// Context for collector operations.
type Context struct {
	client *Client
	db     libmodel.DB
	log    logging.LevelLogger
	ctx    context.Context
}

func (c *Context) canceled() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}
