package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	core "k8s.io/api/core/v1"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// scopedVCenter starts a two-datacenter vCenter simulator. The collector's
// connect() logs into the vAPI REST endpoint of a vCenter, which the simulator
// does not serve, so just enough of it is served here: session login and an
// empty tag list.
func scopedVCenter(t *testing.T) (*simulator.Model, *simulator.Server) {
	t.Helper()
	sim := simulator.VPX()
	sim.Datacenter = 2
	err := sim.Create()
	if err != nil {
		t.Fatalf("create simulator model: %v", err)
	}
	t.Cleanup(sim.Remove)
	// NewServer creates the ServeMux if it is still nil; assign it here so
	// the REST handler is registered before the server accepts connections.
	if sim.Service.ServeMux == nil {
		sim.Service.ServeMux = http.NewServeMux()
	}
	sim.Service.ServeMux.HandleFunc("/rest/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/com/vmware/cis/session":
			_ = json.NewEncoder(w).Encode(map[string]string{"value": "scoped-test-session"})
		case "/rest/com/vmware/cis/tagging/tag":
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": nil})
		default:
			http.NotFound(w, r)
		}
	})
	server := sim.Service.NewServer()
	t.Cleanup(server.Close)
	return sim, server
}

// scopedCollector builds a collector connected to the simulator with the given
// provider settings, and its own object database.
func scopedCollector(t *testing.T, server *simulator.Server, settings map[string]string) (*Collector, *api.Provider, libmodel.DB) {
	t.Helper()
	db := libmodel.New(filepath.Join(t.TempDir(), "inventory.db"), model.All()...)
	err := db.Open(true)
	if err != nil {
		t.Fatalf("open inventory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(true) })
	providerType := api.VSphere
	provider := &api.Provider{
		Spec: api.ProviderSpec{
			Type:     &providerType,
			URL:      server.URL.String(),
			Settings: settings,
		},
	}
	password, _ := server.URL.User.Password()
	secret := &core.Secret{
		Data: map[string][]byte{
			"user":               []byte(server.URL.User.Username()),
			"password":           []byte(password),
			"insecureSkipVerify": []byte("true"),
		},
	}
	return New(db, provider, secret), provider, db
}

// runUntilParity runs the collector's update loop until it reports inventory
// parity, then stops it and waits for it to release the database, so the test
// can read the collected tables without racing the collector.
func runUntilParity(t *testing.T, collector *Collector) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- collector.getUpdates(ctx) }()
	waitParity(t, collector, done)
	cancel()
	err := <-done
	if err != nil {
		t.Fatalf("collector update loop: %v", err)
	}
}

// waitParity blocks until the collector reports inventory parity, failing the
// test if its update loop stops first.
func waitParity(t *testing.T, collector *Collector, done <-chan error) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !collector.HasParity() {
		select {
		case err := <-done:
			t.Fatalf("collector stopped before reaching parity: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("collector never reached inventory parity")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// entitiesUnder returns the simulator entities of the given kinds whose
// parent chain reaches the named datacenter.
func entitiesUnder(t *testing.T, sim *simulator.Model, dcName string, kinds ...string) []mo.Entity {
	t.Helper()
	var dcRef types.ManagedObjectReference
	for _, e := range sim.Map().All("Datacenter") {
		if dc, cast := e.(*simulator.Datacenter); cast && dc.Name == dcName {
			dcRef = dc.Self
			break
		}
	}
	if dcRef.Value == "" {
		t.Fatalf("datacenter %q not found in the simulator", dcName)
	}
	var found []mo.Entity
	for _, kind := range kinds {
		for _, e := range sim.Map().All(kind) {
			// The datacenter itself belongs to its own scope; its parent
			// chain leads to the root folder.
			if e.Entity().Self == dcRef {
				found = append(found, e)
				continue
			}
			for parent := e.Entity().Parent; parent != nil; parent = simulatorParent(sim, parent) {
				if *parent == dcRef {
					found = append(found, e)
					break
				}
			}
		}
	}
	return found
}

// simulatorParent returns the parent of a simulator entity, or nil at the
// root folder.
func simulatorParent(sim *simulator.Model, ref *types.ManagedObjectReference) *types.ManagedObjectReference {
	entity, cast := sim.Map().Get(*ref).(mo.Entity)
	if !cast {
		return nil
	}
	return entity.Entity().Parent
}

// idsUnder returns the managed object IDs of the entities of the given kinds
// whose parent chain reaches the named datacenter.
func idsUnder(t *testing.T, sim *simulator.Model, dcName string, kinds ...string) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, e := range entitiesUnder(t, sim, dcName, kinds...) {
		ids[e.Entity().Self.Value] = true
	}
	return ids
}

// allIDs returns the managed object IDs of every entity of the given kinds in
// the simulator, root folder included.
func allIDs(t *testing.T, sim *simulator.Model, kinds ...string) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, kind := range kinds {
		for _, e := range sim.Map().All(kind) {
			ids[e.Entity().Self.Value] = true
		}
	}
	return ids
}

// collectedIDs lists the IDs stored in one of the collector's tables.
func collectedIDs[T any, P interface {
	*T
	Pk() string
}](t *testing.T, db libmodel.DB) map[string]bool {
	t.Helper()
	var rows []T
	err := db.List(&rows, libmodel.ListOptions{})
	if err != nil {
		t.Fatalf("list collected objects: %v", err)
	}
	ids := make(map[string]bool, len(rows))
	for i := range rows {
		ids[P(&rows[i]).Pk()] = true
	}
	return ids
}

// markTemplate flags one virtual machine under the named datacenter as a
// template and returns its ID.
func markTemplate(t *testing.T, sim *simulator.Model, dcName string) string {
	t.Helper()
	for _, e := range entitiesUnder(t, sim, dcName, "VirtualMachine") {
		vm, cast := e.(*simulator.VirtualMachine)
		if !cast || vm.Config == nil {
			continue
		}
		vm.Config.Template = true
		return vm.Self.Value
	}
	t.Fatalf("no virtual machine to mark as template under datacenter %q", dcName)
	return ""
}

// tableKinds maps a collector table to the simulator entity kinds it collects.
var tableKinds = []struct {
	name  string
	kinds []string
}{
	{"datacenters", []string{"Datacenter"}},
	{"folders", []string{"Folder"}},
	{"clusters", []string{"ClusterComputeResource", "ComputeResource"}},
	{"hosts", []string{"HostSystem"}},
	{"networks", []string{"Network", "DistributedVirtualPortgroup", "VmwareDistributedVirtualSwitch"}},
	{"datastores", []string{"Datastore"}},
	{"resource pools", []string{"ResourcePool"}},
	{"virtual machines", []string{"VirtualMachine"}},
}

// TestDatacenterScopedCollection runs the collector against a
// two-datacenter simulator and asserts, table by table, that the collected
// inventory is exactly the scoped datacenter's objects — and every object
// when the setting does not restrict the provider.
func TestDatacenterScopedCollection(t *testing.T) {
	cases := []struct {
		name     string
		settings map[string]string
		scoped   bool
	}{
		{
			name:     "collects only the scoped datacenter",
			settings: map[string]string{api.Datacenter: "DC0"},
			scoped:   true,
		},
		{
			name:     "empty setting collects every datacenter",
			settings: map[string]string{api.Datacenter: ""},
		},
		{
			name:     "absent setting collects every datacenter",
			settings: nil,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sim, server := scopedVCenter(t)
			dc0Template := markTemplate(t, sim, "DC0")
			dc1Template := markTemplate(t, sim, "DC1")
			collector, provider, db := scopedCollector(t, server, tt.settings)
			runUntilParity(t, collector)

			for _, table := range tableKinds {
				want := allIDs(t, sim, table.kinds...)
				if tt.scoped {
					want = idsUnder(t, sim, "DC0", table.kinds...)
				}
				var got map[string]bool
				switch table.name {
				case "datacenters":
					got = collectedIDs[model.Datacenter](t, db)
				case "folders":
					got = collectedIDs[model.Folder](t, db)
				case "clusters":
					got = collectedIDs[model.Cluster](t, db)
				case "hosts":
					got = collectedIDs[model.Host](t, db)
				case "networks":
					got = collectedIDs[model.Network](t, db)
				case "datastores":
					got = collectedIDs[model.Datastore](t, db)
				case "resource pools":
					got = collectedIDs[model.ResourcePool](t, db)
				case "virtual machines":
					got = collectedIDs[model.VM](t, db)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("collected %s do not match the datacenter scope; missing: %v, extra: %v",
						table.name, missingIDs(want, got), missingIDs(got, want))
				}
			}

			// Inventory datacenter queries list only the scoped datacenter.
			var datacenters []model.Datacenter
			err := db.List(&datacenters, libmodel.ListOptions{})
			if err != nil {
				t.Fatalf("list collected datacenters: %v", err)
			}
			names := make(map[string]bool, len(datacenters))
			for i := range datacenters {
				names[datacenters[i].Name] = true
			}
			wantNames := map[string]bool{"DC0": true, "DC1": true}
			if tt.scoped {
				wantNames = map[string]bool{"DC0": true}
			}
			if !reflect.DeepEqual(names, wantNames) {
				t.Errorf("collected datacenter names = %v, want %v", names, wantNames)
			}

			// Templates are collected like any other virtual machine; the
			// flag is served at a higher detail level than the default.
			var vms []model.VM
			err = db.List(&vms, libmodel.ListOptions{Detail: model.MaxDetail})
			if err != nil {
				t.Fatalf("list collected virtual machines: %v", err)
			}
			templates := map[string]bool{}
			for i := range vms {
				if vms[i].IsTemplate {
					templates[vms[i].ID] = true
				}
			}
			wantTemplates := map[string]bool{dc0Template: true, dc1Template: true}
			if tt.scoped {
				wantTemplates = map[string]bool{dc0Template: true}
			}
			if !reflect.DeepEqual(templates, wantTemplates) {
				t.Errorf("collected templates = %v, want %v", templates, wantTemplates)
			}

			// The setting is never rewritten.
			if got := provider.Spec.Settings[api.Datacenter]; got != tt.settings[api.Datacenter] {
				t.Errorf("datacenter setting rewritten from %q to %q", tt.settings[api.Datacenter], got)
			}
		})
	}
}

// TestDatacenterSettingValidation asserts the connection-test behavior the
// provider's validation relies on: an unresolvable datacenter name fails the
// test with an error naming the datacenter, and a resolvable name passes.
func TestDatacenterSettingValidation(t *testing.T) {
	_, server := scopedVCenter(t)

	t.Run("unresolvable name fails the connection test naming the datacenter", func(t *testing.T) {
		collector, provider, _ := scopedCollector(t, server, map[string]string{api.Datacenter: "no-such-datacenter"})
		_, err := collector.Test()
		if err == nil {
			t.Fatal("connection test succeeded with an unresolvable datacenter name")
		}
		if !strings.Contains(err.Error(), "datacenter 'no-such-datacenter' not found") {
			t.Errorf("connection test error does not name the datacenter: %v", err)
		}
		if got := provider.Spec.Settings[api.Datacenter]; got != "no-such-datacenter" {
			t.Errorf("datacenter setting rewritten from %q to %q", "no-such-datacenter", got)
		}
	})

	t.Run("resolvable name passes the connection test", func(t *testing.T) {
		collector, _, _ := scopedCollector(t, server, map[string]string{api.Datacenter: "DC0"})
		status, err := collector.Test()
		if err != nil {
			t.Fatalf("connection test with a resolvable datacenter: %v", err)
		}
		if status != http.StatusOK {
			t.Errorf("connection test status = %d, want %d", status, http.StatusOK)
		}
	})
}

// TestDatacenterRenameAfterConnect asserts that a datacenter rename after
// connect does not break collection: an update under the renamed datacenter
// is applied, and the setting is not rewritten to the new name.
func TestDatacenterRenameAfterConnect(t *testing.T) {
	sim, server := scopedVCenter(t)
	collector, provider, db := scopedCollector(t, server, map[string]string{api.Datacenter: "DC0"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- collector.getUpdates(ctx) }()
	waitParity(t, collector, done)

	client, err := govmomi.NewClient(ctx, server.URL, true)
	if err != nil {
		t.Fatalf("connect to the simulator: %v", err)
	}
	dcEntity := entitiesUnder(t, sim, "DC0", "Datacenter")[0]
	vmEntity := entitiesUnder(t, sim, "DC0", "VirtualMachine")[0]
	dc := object.NewDatacenter(client.Client, dcEntity.Entity().Self)
	task, err := dc.Rename(ctx, "DC0-renamed")
	if err != nil {
		t.Fatalf("rename the scoped datacenter: %v", err)
	}
	err = task.Wait(ctx)
	if err != nil {
		t.Fatalf("rename the scoped datacenter: %v", err)
	}

	// Change an object under the renamed datacenter and wait for the
	// collector to apply the update to its object database.
	vm := object.NewVirtualMachine(client.Client, vmEntity.Entity().Self)
	task, err = vm.Rename(ctx, "renamed-by-test")
	if err != nil {
		t.Fatalf("rename a virtual machine under the renamed datacenter: %v", err)
	}
	err = task.Wait(ctx)
	if err != nil {
		t.Fatalf("rename a virtual machine under the renamed datacenter: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		collected := &model.VM{Base: model.Base{ID: vmEntity.Entity().Self.Value}}
		err = db.Get(collected)
		if err == nil && collected.Name == "renamed-by-test" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatalf("collector never applied the update made after the datacenter rename; name=%q err=%v", collected.Name, err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	err = <-done
	if err != nil {
		t.Fatalf("collector update loop: %v", err)
	}

	// The scoped datacenter is still collected under its reference, and the
	// setting is not rewritten to the new name.
	err = db.Get(&model.Datacenter{Base: model.Base{ID: dcEntity.Entity().Self.Value}})
	if err != nil {
		t.Fatalf("scoped datacenter no longer collected after the rename: %v", err)
	}
	if got := provider.Spec.Settings[api.Datacenter]; got != "DC0" {
		t.Errorf("datacenter setting rewritten from %q to %q", "DC0", got)
	}
}

// missingIDs returns the keys of want that are absent from got, for failure
// messages.
func missingIDs(want, got map[string]bool) []string {
	var missing []string
	for id := range want {
		if !got[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 10 {
		missing = append(missing[:10], fmt.Sprintf("... and %d more", len(missing)-10))
	}
	return missing
}
