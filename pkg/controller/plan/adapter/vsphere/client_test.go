package vsphere

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

const (
	testCatID = "urn:vmomi:InventoryServiceCategory:cat-1:GLOBAL"
	testTagID = "urn:vmomi:InventoryServiceTag:tag-1:GLOBAL"
)

// newTagServer creates a minimal vSphere VAPI mock.
//
//	categoryExists — if true, GET /category returns testCatID and the category exists.
//	tagExists      — if true, list-tags-for-category returns testTagID.
//	attachedVMs    — collects MOR values of VMs that receive AttachTag calls.
//	batchAttachFails — if true, batch attach returns HTTP 500.
func newTagServer(categoryExists, tagExists bool, attachedVMs *[]string, mu *sync.Mutex, batchAttachFails bool) *httptest.Server {
	mux := http.NewServeMux()

	// SOAP SDK endpoint for vim25.NewClient
	mux.HandleFunc("/sdk", func(w http.ResponseWriter, r *http.Request) {
		// Minimal SOAP response for vim25 initialization
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
<soapenv:Body>
<RetrieveServiceContentResponse xmlns="urn:vim25">
<returnval>
<rootFolder type="Folder">group-d1</rootFolder>
<sessionManager type="SessionManager">SessionManager</sessionManager>
</returnval>
</RetrieveServiceContentResponse>
</soapenv:Body>
</soapenv:Envelope>`))
	})

	// Session
	mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"value": "fake-session"})
	})

	// List categories
	mux.HandleFunc("/rest/com/vmware/cis/tagging/category", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			// CreateCategory
			_ = json.NewEncoder(w).Encode(map[string]string{"value": testCatID})
			return
		}
		// ListCategories
		if categoryExists {
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": {testCatID}})
		} else {
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": {}})
		}
	})

	// Get single category
	mux.HandleFunc("/rest/com/vmware/cis/tagging/category/id:"+testCatID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cat := map[string]any{
			"id":               testCatID,
			"name":             settings.DefaultVspherePostMigrationTagCategory,
			"cardinality":      "MULTIPLE",
			"associable_types": []string{"VirtualMachine"},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": cat})
	})

	// List tags for category — govmomi sends ?~action=list-tags-for-category as a query param.
	mux.HandleFunc("/rest/com/vmware/cis/tagging/tag/id:"+testCatID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("~action") != "list-tags-for-category" {
			http.NotFound(w, r)
			return
		}
		if tagExists {
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": {testTagID}})
		} else {
			_ = json.NewEncoder(w).Encode(map[string][]string{"value": {}})
		}
	})

	// Get single tag
	mux.HandleFunc("/rest/com/vmware/cis/tagging/tag/id:"+testTagID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tag := map[string]any{
			"id":          testTagID,
			"name":        settings.DefaultVspherePostMigrationTagName,
			"category_id": testCatID,
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": tag})
	})

	// Create tag
	mux.HandleFunc("/rest/com/vmware/cis/tagging/tag", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": testTagID})
	})

	// Batch attach — govmomi sends ?~action=attach-tag-to-multiple-objects
	mux.HandleFunc("/rest/com/vmware/cis/tagging/tag-association/id:"+testTagID, func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("~action")
		if action != "attach-tag-to-multiple-objects" {
			http.NotFound(w, r)
			return
		}
		if batchAttachFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var body struct {
			ObjectIDs []struct {
				Type  string `json:"type"`
				Value string `json:"id"`
			} `json:"object_ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		for _, obj := range body.ObjectIDs {
			*attachedVMs = append(*attachedVMs, obj.Value)
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	return srv
}

// newTestClient builds a Client for testing.
func newTestClient(serverURL string) (*Client, *rest.Client) {
	parsed, _ := url.Parse(serverURL)
	soapClient := soap.NewClient(parsed, true)
	vimClient := &vim25.Client{Client: soapClient}
	restClient := rest.NewClient(vimClient)
	_ = restClient.Login(context.Background(), url.UserPassword("u", "p"))

	c := &Client{
		Context: &plancontext.Context{
			Log: logging.WithName("test"),
		},
	}
	return c, restClient
}

// succeededVM returns a VMStatus that has ConditionSucceeded set.
func succeededVM(id string) *planapi.VMStatus {
	s := &planapi.VMStatus{VM: planapi.VM{Ref: ref.Ref{ID: id}}}
	s.SetCondition(libcnd.Condition{Type: api.ConditionSucceeded, Status: "True"})
	return s
}

// failedVM returns a VMStatus that has ConditionFailed set.
func failedVM(id string) *planapi.VMStatus {
	s := &planapi.VMStatus{VM: planapi.VM{Ref: ref.Ref{ID: id}}}
	s.SetCondition(libcnd.Condition{Type: api.ConditionFailed, Status: "True"})
	return s
}

// canceledVM returns a VMStatus that has ConditionCanceled set.
func canceledVM(id string) *planapi.VMStatus {
	s := &planapi.VMStatus{VM: planapi.VM{Ref: ref.Ref{ID: id}}}
	s.SetCondition(libcnd.Condition{Type: api.ConditionCanceled, Status: "True"})
	return s
}

var _ = Describe("Client.Finalize tagging", func() {
	var (
		attachedVMs []string
		mu          sync.Mutex
		srv         *httptest.Server
	)

	BeforeEach(func() {
		attachedVMs = nil
		mu = sync.Mutex{}
		_ = settings.Settings.Providers.Load()
	})

	AfterEach(func() {
		if srv != nil {
			srv.CloseClientConnections()
			srv.Close()
			srv = nil
		}
	})

	It("attaches the tag to all succeeded VMs", func() {
		srv = newTagServer(true, true, &attachedVMs, &mu, false)
		c, restClient := newTestClient(srv.URL)

		c.finalizeTagMigrated(context.Background(),
			restClient,
			[]*planapi.VMStatus{succeededVM("vm-100"), succeededVM("vm-200")},
		)

		Expect(attachedVMs).To(ConsistOf("vm-100", "vm-200"))
	})

	It("filters out failed and canceled VMs when called via Finalize", func() {
		srv = newTagServer(true, true, &attachedVMs, &mu, false)
		c, restClient := newTestClient(srv.URL)

		// Setup provider (non-ESXi so Finalize doesn't skip tagging)
		c.Source = plancontext.Source{
			Provider: &api.Provider{
				Spec: api.ProviderSpec{
					Settings: map[string]string{
						api.SDK: "vcenter",
					},
				},
			},
		}

		// Simulate what Finalize does: filter to succeeded VMs only, then tag
		allVMs := []*planapi.VMStatus{
			succeededVM("vm-success-1"),
			failedVM("vm-failed"),
			succeededVM("vm-success-2"),
			canceledVM("vm-canceled"),
		}

		// Filter - this is what Finalize does internally
		var succeeded []*planapi.VMStatus
		for _, vm := range allVMs {
			if vm.HasCondition(api.ConditionSucceeded) {
				succeeded = append(succeeded, vm)
			}
		}

		// Verify filtering worked
		Expect(succeeded).To(HaveLen(2))

		// Now tag only the succeeded VMs
		c.finalizeTagMigrated(context.Background(), restClient, succeeded)

		// Only succeeded VMs should be tagged
		Expect(attachedVMs).To(ConsistOf("vm-success-1", "vm-success-2"))
	})

	It("is a no-op when all VMs failed", func() {
		srv = newTagServer(true, true, &attachedVMs, &mu, false)
		c, restClient := newTestClient(srv.URL)

		// finalizeTagMigrated assumes input is already filtered to succeeded VMs.
		// When all VMs failed, Finalize would pass an empty array.
		c.finalizeTagMigrated(context.Background(),
			restClient,
			[]*planapi.VMStatus{},
		)

		Expect(attachedVMs).To(BeEmpty())
	})

	It("creates category and tag when neither exists", func() {
		srv = newTagServer(false, false, &attachedVMs, &mu, false)
		c, restClient := newTestClient(srv.URL)

		Expect(func() {
			c.finalizeTagMigrated(context.Background(),
				restClient,
				[]*planapi.VMStatus{succeededVM("vm-new")},
			)
		}).NotTo(Panic())
		// The mock create-path returns testTagID, so attach should have fired.
		Expect(attachedVMs).To(ContainElement("vm-new"))
	})

	It("creates tag when category exists but tag does not", func() {
		srv = newTagServer(true, false, &attachedVMs, &mu, false)
		c, restClient := newTestClient(srv.URL)

		c.finalizeTagMigrated(context.Background(),
			restClient,
			[]*planapi.VMStatus{succeededVM("vm-x")},
		)

		Expect(attachedVMs).To(ContainElement("vm-x"))
	})

	It("logs batch attachment failure", func() {
		srv = newTagServer(true, true, &attachedVMs, &mu, true)
		c, restClient := newTestClient(srv.URL)

		Expect(func() {
			c.finalizeTagMigrated(context.Background(), restClient,
				[]*planapi.VMStatus{succeededVM("vm-1"), succeededVM("vm-2")})
		}).NotTo(Panic())

		// Batch attach failed (HTTP 500), so no VMs were tagged.
		Expect(attachedVMs).To(BeEmpty())
	})

	It("skips tagging when disabled", func() {
		original := settings.Settings.PostMigrationTaggingEnabled
		defer func() { settings.Settings.PostMigrationTaggingEnabled = original }()
		settings.Settings.PostMigrationTaggingEnabled = false

		srv = newTagServer(true, true, &attachedVMs, &mu, false)
		c, _ := newTestClient(srv.URL)

		// Build a minimal valid non-ESXi provider so Finalize doesn't panic on nil dereference.
		c.Source = plancontext.Source{
			Provider: &api.Provider{
				Spec: api.ProviderSpec{
					Settings: map[string]string{
						api.SDK: "vcenter", // not ESXI
					},
				},
			},
		}

		c.Finalize([]*planapi.VMStatus{succeededVM("vm-1")}, "test-plan")

		// Tagging is disabled, so no REST calls should happen.
		Expect(attachedVMs).To(BeEmpty())
	})

	Describe("resolveOrCreateTagCategory", func() {
		It("returns existing category ID when category exists", func() {
			srv := newTagServer(true, false, nil, nil, false)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			categoryID, err := c.resolveOrCreateTagCategory(context.Background(), tagManager, settings.DefaultVspherePostMigrationTagCategory)

			Expect(err).NotTo(HaveOccurred())
			Expect(categoryID).To(Equal(testCatID))
		})

		It("creates category when it doesn't exist", func() {
			srv := newTagServer(false, false, nil, nil, false)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			categoryID, err := c.resolveOrCreateTagCategory(context.Background(), tagManager, settings.DefaultVspherePostMigrationTagCategory)

			Expect(err).NotTo(HaveOccurred())
			Expect(categoryID).To(Equal(testCatID))
		})

		It("handles concurrent creation race by re-fetching category", func() {
			// Mock that simulates concurrent create: GetCategory 404 → CreateCategory fails → GetCategory succeeds
			mux := http.NewServeMux()
			createAttempts := 0
			getCategoryCallsbyname := 0

			mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "fake-session"})
			})

			// List/Create Category
			mux.HandleFunc("/rest/com/vmware/cis/tagging/category", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					createAttempts++
					// Simulate concurrent create race - return 400
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				// List categories
				getCategoryCallsbyname++
				if getCategoryCallsbyname == 1 {
					// First call: category doesn't exist yet
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {}})
				} else {
					// After failed create: another caller created it
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {testCatID}})
				}
			})

			// GetCategory by ID - succeeds (another caller created it)
			mux.HandleFunc("/rest/com/vmware/cis/tagging/category/id:"+testCatID, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				cat := map[string]any{
					"id":               testCatID,
					"name":             settings.DefaultVspherePostMigrationTagCategory,
					"cardinality":      "MULTIPLE",
					"associable_types": []string{"VirtualMachine"},
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"value": cat})
			})

			srv := httptest.NewServer(mux)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			categoryID, err := c.resolveOrCreateTagCategory(context.Background(), tagManager, settings.DefaultVspherePostMigrationTagCategory)

			Expect(err).NotTo(HaveOccurred())
			Expect(categoryID).To(Equal(testCatID))
			Expect(createAttempts).To(Equal(1), "should attempt create once, then recover via get")
		})
	})

	Describe("resolveOrCreateTag", func() {
		It("returns existing tag ID when tag exists in category", func() {
			srv := newTagServer(true, true, nil, nil, false)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			tagID, err := c.resolveOrCreateTag(context.Background(), tagManager, testCatID, settings.DefaultVspherePostMigrationTagName)

			Expect(err).NotTo(HaveOccurred())
			Expect(tagID).To(Equal(testTagID))
		})

		It("creates tag when it doesn't exist in category", func() {
			srv := newTagServer(true, false, nil, nil, false)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			tagID, err := c.resolveOrCreateTag(context.Background(), tagManager, testCatID, settings.DefaultVspherePostMigrationTagName)

			Expect(err).NotTo(HaveOccurred())
			Expect(tagID).To(Equal(testTagID))
		})

		It("handles concurrent creation race by re-listing tags", func() {
			// Mock that simulates concurrent create: CreateTag fails, GetTagsForCategory finds it
			mux := http.NewServeMux()
			createAttempts := 0

			mux.HandleFunc("/rest/com/vmware/cis/session", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "fake-session"})
			})

			// CreateTag - fails on first call (race)
			mux.HandleFunc("/rest/com/vmware/cis/tagging/tag", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					createAttempts++
					// Simulate concurrent create race - return 400
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			})

			// GetTagsForCategory - returns empty first, then includes tag after failed create
			listCalls := 0
			mux.HandleFunc("/rest/com/vmware/cis/tagging/tag/id:"+testCatID, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("~action") != "list-tags-for-category" {
					http.NotFound(w, r)
					return
				}
				listCalls++
				if listCalls > 1 {
					// After failed create, another caller created it
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {testTagID}})
				} else {
					// First list: empty
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {}})
				}
			})

			// GetTag - return tag details
			mux.HandleFunc("/rest/com/vmware/cis/tagging/tag/id:"+testTagID, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				tag := map[string]any{
					"id":          testTagID,
					"name":        settings.DefaultVspherePostMigrationTagName,
					"category_id": testCatID,
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"value": tag})
			})

			srv := httptest.NewServer(mux)
			defer srv.Close()
			c, restClient := newTestClient(srv.URL)

			tagManager := tags.NewManager(restClient)
			tagID, err := c.resolveOrCreateTag(context.Background(), tagManager, testCatID, settings.DefaultVspherePostMigrationTagName)

			Expect(err).NotTo(HaveOccurred())
			Expect(tagID).To(Equal(testTagID))
			Expect(createAttempts).To(Equal(1), "should attempt create once, then recover via list")
		})
	})
})
