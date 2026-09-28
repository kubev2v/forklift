package vsphere

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	liburl "net/url"
	"os"
	"path/filepath"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var _ = Describe("vSphere collector", func() {
	collector := Collector{}
	url, _ := liburl.Parse("https://fake.com/sdk")
	vimClient := &vim25.Client{
		Client:         soap.NewClient(url, false),
		ServiceContent: types.ServiceContent{},
	}
	collector.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}

	table.DescribeTable("should", func(version string, matchTpm gtypes.GomegaMatcher) {
		collector.client.ServiceContent.About.ApiVersion = version
		Expect(collector.vmPathSet()).Should(matchTpm)
	},
		table.Entry("not collect TPM from vSphere < 6.7", "6.5", Not(ContainElements(fTpmPresent))),
		table.Entry("collect TPM from vSphere 6.7", "6.7", ContainElements(fTpmPresent)),
		table.Entry("collect TPM from vSphere > 6.7", "7.0", ContainElements(fTpmPresent)),
	)
})

var _ = Describe("apply", func() {
	var (
		collector *Collector
		db        libmodel.DB
		tmpDir    string
		ctx       context.Context
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vsphere-collector-test")
		Expect(err).NotTo(HaveOccurred())
		dbPath := filepath.Join(tmpDir, "test.db")
		db = libmodel.New(dbPath, model.All()...)
		err = db.Open(true)
		Expect(err).NotTo(HaveOccurred())

		// With restClient=nil, getVMsWithTags returns an empty map
		// without making any network calls.
		collector = &Collector{
			log: logging.WithName("test"),
			db:  db,
		}
		ctx = context.Background()
	})

	AfterEach(func() {
		if db != nil {
			_ = db.Close(true)
		}
		_ = os.RemoveAll(tmpDir)
	})

	It("should insert a VM on Enter event", func() {
		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-100"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.VM{Base: model.Base{ID: "vm-100"}})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should update a VM on Modify event", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-200"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindModify,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-200"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		vm := &model.VM{Base: model.Base{ID: "vm-200"}}
		err = db.Get(vm)
		Expect(err).NotTo(HaveOccurred())
		Expect(vm.Revision).To(BeNumerically(">", 1))
	})

	It("should delete a VM on Leave event", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-300"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindLeave,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-300"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.VM{Base: model.Base{ID: "vm-300"}})
		Expect(err).To(HaveOccurred())
	})

	It("should return error on Modify for non-existent VM", func() {
		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindModify,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-nonexistent"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).To(HaveOccurred())
		_ = tx.End()
	})

	It("should process Enter and Leave events in the same batch without error", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-leave"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-enter"},
			},
			{
				Kind: types.ObjectUpdateKindLeave,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-leave"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.VM{Base: model.Base{ID: "vm-enter"}})
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.VM{Base: model.Base{ID: "vm-leave"}})
		Expect(err).To(HaveOccurred())
	})

	It("should handle non-VirtualMachine types gracefully", func() {
		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: Folder, Value: "folder-1"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.Folder{Base: model.Base{ID: "folder-1"}})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should insert multiple VMs on batch Enter events", func() {
		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-batch-1"},
			},
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-batch-2"},
			},
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-batch-3"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		for _, id := range []string{"vm-batch-1", "vm-batch-2", "vm-batch-3"} {
			err = db.Get(&model.VM{Base: model.Base{ID: id}})
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("should upsert on duplicate Enter event", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-dup"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-dup"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		vm := &model.VM{Base: model.Base{ID: "vm-dup"}}
		err = db.Get(vm)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("apply with tags", func() {
	const tagID = "urn:vmomi:InventoryServiceTag:tag-1:GLOBAL"
	const tagPath = "/rest/com/vmware/cis/tagging/tag"

	var (
		collector    *Collector
		db           libmodel.DB
		tmpDir       string
		ctx          context.Context
		server       *httptest.Server
		tagAssocHits int32
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "vsphere-collector-tags-test")
		Expect(err).NotTo(HaveOccurred())
		dbPath := filepath.Join(tmpDir, "test.db")
		db = libmodel.New(dbPath, model.All()...)
		err = db.Open(true)
		Expect(err).NotTo(HaveOccurred())

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			path := r.URL.Path
			switch {
			case path == "/rest/com/vmware/cis/session":
				_, _ = fmt.Fprintf(w, `{"value":"fake-session"}`)
			case path == tagPath && r.Method == http.MethodGet:
				_, _ = fmt.Fprintf(w, `{"value":[%q]}`, tagID)
			case path == tagPath+"/id:"+tagID:
				_, _ = fmt.Fprintf(w, `{"value":{"id":%q,"name":"test-tag","description":"test","category_id":"cat-1","used_by":[]}}`, tagID)
			case path == "/rest/com/vmware/cis/tagging/tag-association":
				atomic.AddInt32(&tagAssocHits, 1)
				_, _ = fmt.Fprintf(w, `{"value":[{"tag_id":%q,"object_ids":[{"id":"vm-tag-enter","type":"VirtualMachine"},{"id":"vm-tag-modify","type":"VirtualMachine"}]}]}`, tagID)
			default:
				http.NotFound(w, r)
			}
		})

		server = httptest.NewServer(handler)
		server.Config.SetKeepAlivesEnabled(false)

		serverURL, err := liburl.Parse(server.URL)
		Expect(err).NotTo(HaveOccurred())

		soapClient := soap.NewClient(serverURL, true)
		soapClient.DefaultTransport().DisableKeepAlives = true
		vimClient := &vim25.Client{Client: soapClient}
		restClient := rest.NewClient(vimClient)
		err = restClient.Login(context.Background(), liburl.UserPassword("admin", "pass"))
		Expect(err).NotTo(HaveOccurred())

		collector = &Collector{
			log:        logging.WithName("test"),
			db:         db,
			restClient: restClient,
		}
		atomic.StoreInt32(&tagAssocHits, 0)
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.CloseClientConnections()
			server.Close()
		}
		if db != nil {
			_ = db.Close(true)
		}
		_ = os.RemoveAll(tmpDir)
	})

	It("should apply tags on Enter event", func() {
		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-enter"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		vm := &model.VM{Base: model.Base{ID: "vm-tag-enter"}}
		err = db.Get(vm)
		Expect(err).NotTo(HaveOccurred())
		Expect(vm.Tags).To(HaveLen(1))
		Expect(vm.Tags[0].Name).To(Equal("test-tag"))
	})

	It("should not request tag associations on Leave event", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-tag-leave"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindLeave,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-leave"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		err = db.Get(&model.VM{Base: model.Base{ID: "vm-tag-leave"}})
		Expect(err).To(HaveOccurred())

		// No tag-association requests should have been made for Leave events.
		Expect(atomic.LoadInt32(&tagAssocHits)).To(Equal(int32(0)))
	})

	It("should apply tags on Modify event", func() {
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-tag-modify"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindModify,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-modify"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		vm := &model.VM{Base: model.Base{ID: "vm-tag-modify"}}
		err = db.Get(vm)
		Expect(err).NotTo(HaveOccurred())
		Expect(vm.Tags).To(HaveLen(1))
	})

	It("should continue processing when tag fetch fails", func() {
		// Replace the mock server with one that returns 500 on tag listing
		server.Close()
		failHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/rest/com/vmware/cis/session":
				_, _ = fmt.Fprintf(w, `{"value":"fake-session"}`)
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		})
		failServer := httptest.NewServer(failHandler)
		failServer.Config.SetKeepAlivesEnabled(false)
		defer func() {
			failServer.CloseClientConnections()
			failServer.Close()
		}()

		serverURL, err := liburl.Parse(failServer.URL)
		Expect(err).NotTo(HaveOccurred())
		soapClient := soap.NewClient(serverURL, true)
		vimClient := &vim25.Client{Client: soapClient}
		restClient := rest.NewClient(vimClient)
		err = restClient.Login(context.Background(), liburl.UserPassword("admin", "pass"))
		Expect(err).NotTo(HaveOccurred())
		collector.restClient = restClient

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-fail"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		// VM should be inserted even though tag fetch failed
		vm := &model.VM{Base: model.Base{ID: "vm-tag-fail"}}
		err = db.Get(vm)
		Expect(err).NotTo(HaveOccurred())
		// Tags should be empty since fetch failed
		Expect(vm.Tags).To(BeEmpty())
	})

	It("should process Enter and Leave in a batch without data loss when tags are enabled", func() {
		// Pre-insert the VM that will be removed by Leave.
		err := db.Insert(&model.VM{Base: model.Base{ID: "vm-tag-leave-batch"}})
		Expect(err).NotTo(HaveOccurred())

		updates := []types.ObjectUpdate{
			{
				Kind: types.ObjectUpdateKindEnter,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-enter"},
			},
			{
				Kind: types.ObjectUpdateKindLeave,
				Obj:  types.ManagedObjectReference{Type: VirtualMachine, Value: "vm-tag-leave-batch"},
			},
		}
		tx, err := db.Begin()
		Expect(err).NotTo(HaveOccurred())
		err = collector.apply(ctx, tx, updates)
		Expect(err).NotTo(HaveOccurred())
		err = tx.Commit()
		Expect(err).NotTo(HaveOccurred())

		// The entered VM must survive the transaction — this is the
		// data-loss scenario MTV-6073 fixes.
		err = db.Get(&model.VM{Base: model.Base{ID: "vm-tag-enter"}})
		Expect(err).NotTo(HaveOccurred())

		// The leaving VM must be deleted.
		err = db.Get(&model.VM{Base: model.Base{ID: "vm-tag-leave-batch"}})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("HasParity", func() {
	It("should return false initially", func() {
		collector := &Collector{}
		Expect(collector.HasParity()).To(BeFalse())
	})

	It("should return true after setting parity", func() {
		collector := &Collector{}
		collector.parity = true
		Expect(collector.HasParity()).To(BeTrue())
	})

	It("should return false after Reset clears parity", func() {
		collector := &Collector{log: logging.WithName("test")}
		collector.parity = true
		collector.Reset()
		Expect(collector.HasParity()).To(BeFalse())
	})
})
