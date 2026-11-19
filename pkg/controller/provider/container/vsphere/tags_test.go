package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
)

var _ = Describe("Tag fetching", func() {
	var (
		server     *httptest.Server
		collector  *Collector
		restClient *rest.Client
		ctx        context.Context
		getTagReqs int32
	)

	newMockVCenter := func(numTags int, failTagID string) *httptest.Server {
		tagPrefix := "/rest/com/vmware/cis/tagging/tag"
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			switch {
			case path == "/rest/com/vmware/cis/session":
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusOK)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"value": "fake-session"})

			case path == tagPrefix && r.Method == http.MethodGet:
				ids := make([]string, numTags)
				for i := range ids {
					ids[i] = fmt.Sprintf("urn:vmomi:InventoryServiceTag:tag-%d:GLOBAL", i)
				}
				_ = json.NewEncoder(w).Encode(map[string][]string{"value": ids})

			case len(path) > len(tagPrefix)+4 && path[:len(tagPrefix)+4] == tagPrefix+"/id:":
				atomic.AddInt32(&getTagReqs, 1)
				tagID := path[len(tagPrefix)+4:]
				if tagID == failTagID {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				tag := map[string]interface{}{
					"id":          tagID,
					"name":        "name-" + tagID,
					"description": "desc-" + tagID,
					"category_id": "urn:vmomi:InventoryServiceCategory:cat-1:GLOBAL",
					"used_by":     []string{},
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"value": tag})

			case path == "/rest/com/vmware/cis/tagging/tag-association" && r.Method == http.MethodPost:
				var req struct {
					TagIDs []string `json:"tag_ids"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)

				type attachedObj struct {
					Type  string `json:"type"`
					Value string `json:"id"`
				}
				type result struct {
					TagID     string        `json:"tag_id"`
					ObjectIDs []attachedObj `json:"object_ids"`
				}

				results := make([]result, 0, len(req.TagIDs))
				for i, tagID := range req.TagIDs {
					switch i % 3 {
					case 0:
						results = append(results, result{
							TagID: tagID,
							ObjectIDs: []attachedObj{
								{Type: "VirtualMachine", Value: fmt.Sprintf("vm-%d", i%5)},
							},
						})
					case 1:
						results = append(results, result{
							TagID: tagID,
							ObjectIDs: []attachedObj{
								{Type: "HostSystem", Value: fmt.Sprintf("host-%d", i%3)},
							},
						})
					default:
						results = append(results, result{
							TagID: tagID,
							ObjectIDs: []attachedObj{
								{Type: "VirtualMachine", Value: fmt.Sprintf("vm-%d", i%5)},
								{Type: "HostSystem", Value: "host-0"},
							},
						})
					}
				}
				_ = json.NewEncoder(w).Encode(map[string][]result{"value": results})

			default:
				http.NotFound(w, r)
			}
		})

		srv := httptest.NewServer(handler)
		srv.Config.SetKeepAlivesEnabled(false)
		return srv
	}

	createCollector := func(serverURL string) *Collector {
		parsedURL, _ := url.Parse(serverURL)
		soapClient := soap.NewClient(parsedURL, true)
		soapClient.DefaultTransport().DisableKeepAlives = true
		vimClient := &vim25.Client{Client: soapClient}
		restClient = rest.NewClient(vimClient)
		userInfo := url.UserPassword("admin", "pass")
		err := restClient.Login(context.Background(), userInfo)
		Expect(err).NotTo(HaveOccurred())
		log := logging.WithName("test")

		return &Collector{
			restClient: restClient,
			log:        log,
			secret: &core.Secret{
				Data: map[string][]byte{
					"user":     []byte("admin"),
					"password": []byte("pass"),
				},
			},
		}
	}

	BeforeEach(func() {
		atomic.StoreInt32(&getTagReqs, 0)
		ctx = context.Background()
		_ = os.Unsetenv(settings.VsphereTagFetchConcurrencyEv)
		settings.Settings.VsphereTagFetchConcurrency = settings.DefaultVsphereTagFetchConcurrency
	})

	AfterEach(func() {
		if server != nil {
			server.CloseClientConnections()
			server.Close()
			server = nil
		}
		restClient = nil
	})

	Describe("env var configuration via settings pkg", func() {
		AfterEach(func() {
			_ = os.Unsetenv(settings.VsphereTagFetchConcurrencyEv)
		})

		It("uses default when env is unset", func() {
			Expect(os.Unsetenv(settings.VsphereTagFetchConcurrencyEv)).To(Succeed())
			err := settings.Settings.Providers.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Settings.VsphereTagFetchConcurrency).To(Equal(settings.DefaultVsphereTagFetchConcurrency))
		})

		It("parses valid env var", func() {
			Expect(os.Setenv(settings.VsphereTagFetchConcurrencyEv, "25")).To(Succeed())
			err := settings.Settings.Providers.Load()
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.Settings.VsphereTagFetchConcurrency).To(Equal(25))
		})

		It("returns error for invalid env var values", func() {
			for _, val := range []string{"0", "-5", "abc"} {
				Expect(os.Setenv(settings.VsphereTagFetchConcurrencyEv, val)).To(Succeed())
				err := settings.Settings.Providers.Load()
				Expect(err).To(HaveOccurred(), "should fail for value %q", val)
			}
		})
	})

	Describe("non-VM attachment filtering", func() {
		It("only returns tags attached to VirtualMachine objects", func() {
			server = newMockVCenter(9, "")
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).NotTo(HaveOccurred())

			// With 9 tags: indices 0,3,6 attached to VM only; 2,5,8 attached to VM+Host
			// Indices 1,4,7 attached to HostSystem only -> filtered out
			// So 6 tags are VM-relevant
			totalVMTags := 0
			for _, tags := range result {
				totalVMTags += len(tags)
			}
			Expect(totalVMTags).To(Equal(6))

			// GetTag should only be called for VM-relevant tags (6), not all 9
			Expect(int(atomic.LoadInt32(&getTagReqs))).To(Equal(6))
		})

		It("returns empty map when no tags are attached to VMs", func() {
			type obj struct {
				Type  string `json:"type"`
				Value string `json:"id"`
			}
			type res struct {
				TagID     string `json:"tag_id"`
				ObjectIDs []obj  `json:"object_ids"`
			}
			tag1 := "urn:vmomi:InventoryServiceTag:no-vm-1:GLOBAL"
			tag2 := "urn:vmomi:InventoryServiceTag:no-vm-2:GLOBAL"
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/com/vmware/cis/session":
					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"value": "s"})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag" && r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {tag1, tag2}})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag-association":
					results := []res{
						{TagID: tag1, ObjectIDs: []obj{{Type: "HostSystem", Value: "host-1"}}},
						{TagID: tag2, ObjectIDs: []obj{{Type: "Datastore", Value: "ds-1"}}},
					}
					_ = json.NewEncoder(w).Encode(map[string][]res{"value": results})
				default:
					http.NotFound(w, r)
				}
			})
			srv := httptest.NewServer(handler)
			srv.Config.SetKeepAlivesEnabled(false)
			server = srv
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Describe("GetTag failure handling", func() {
		It("returns error when a GetTag call fails", func() {
			server = newMockVCenter(6, "urn:vmomi:InventoryServiceTag:tag-0:GLOBAL")
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("CIS REST session re-auth", func() {
		const tagID = "urn:vmomi:InventoryServiceTag:tag-0:GLOBAL"

		writeTagAssociation := func(w http.ResponseWriter) {
			type attachedObj struct {
				Type  string `json:"type"`
				Value string `json:"id"`
			}
			type result struct {
				TagID     string        `json:"tag_id"`
				ObjectIDs []attachedObj `json:"object_ids"`
			}
			_ = json.NewEncoder(w).Encode(map[string][]result{
				"value": {{
					TagID: tagID,
					ObjectIDs: []attachedObj{
						{Type: "VirtualMachine", Value: "vm-1"},
					},
				}},
			})
		}

		writeTag := func(w http.ResponseWriter) {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"value": map[string]interface{}{
					"id":          tagID,
					"name":        "name-" + tagID,
					"description": "desc",
					"category_id": "urn:vmomi:InventoryServiceCategory:cat-1:GLOBAL",
					"used_by":     []string{},
				},
			})
		}

		It("re-authenticates and retries when ListTags returns 401", func() {
			var listTagsHits int32
			var loginHits int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/com/vmware/cis/session":
					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}
					atomic.AddInt32(&loginHits, 1)
					_ = json.NewEncoder(w).Encode(map[string]string{"value": fmt.Sprintf("session-%d", loginHits)})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag" && r.Method == http.MethodGet:
					if atomic.AddInt32(&listTagsHits, 1) == 1 {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {tagID}})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag-association":
					writeTagAssociation(w)
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag/id:"+tagID:
					writeTag(w)
				default:
					http.NotFound(w, r)
				}
			})
			srv := httptest.NewServer(handler)
			srv.Config.SetKeepAlivesEnabled(false)
			server = srv
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("vm-1"))
			Expect(atomic.LoadInt32(&listTagsHits)).To(Equal(int32(2)))
			// Initial Login in createCollector + re-auth Login after 401.
			Expect(atomic.LoadInt32(&loginHits)).To(Equal(int32(2)))
		})

		It("re-authenticates and retries when ListAttachedObjectsOnTags returns 401", func() {
			var assocHits int32
			var loginHits int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/com/vmware/cis/session":
					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}
					atomic.AddInt32(&loginHits, 1)
					_ = json.NewEncoder(w).Encode(map[string]string{"value": fmt.Sprintf("session-%d", loginHits)})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag" && r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string][]string{"value": {tagID}})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag-association":
					if atomic.AddInt32(&assocHits, 1) == 1 {
						http.Error(w, "unauthorized", http.StatusUnauthorized)
						return
					}
					writeTagAssociation(w)
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag/id:"+tagID:
					writeTag(w)
				default:
					http.NotFound(w, r)
				}
			})
			srv := httptest.NewServer(handler)
			srv.Config.SetKeepAlivesEnabled(false)
			server = srv
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("vm-1"))
			Expect(atomic.LoadInt32(&assocHits)).To(Equal(int32(2)))
			Expect(atomic.LoadInt32(&loginHits)).To(Equal(int32(2)))
		})

		It("returns error when re-auth Login fails after 401", func() {
			var loginHits int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/com/vmware/cis/session":
					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}
					if atomic.AddInt32(&loginHits, 1) == 1 {
						_ = json.NewEncoder(w).Encode(map[string]string{"value": "session-1"})
						return
					}
					http.Error(w, "unauthorized", http.StatusUnauthorized)
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag" && r.Method == http.MethodGet:
					http.Error(w, "unauthorized", http.StatusUnauthorized)
				default:
					http.NotFound(w, r)
				}
			})
			srv := httptest.NewServer(handler)
			srv.Config.SetKeepAlivesEnabled(false)
			server = srv
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(atomic.LoadInt32(&loginHits)).To(Equal(int32(2)))
		})

		It("does not re-authenticate on non-401 errors", func() {
			var listTagsHits int32
			var loginHits int32
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/rest/com/vmware/cis/session":
					if r.Method == http.MethodDelete {
						w.WriteHeader(http.StatusOK)
						return
					}
					atomic.AddInt32(&loginHits, 1)
					_ = json.NewEncoder(w).Encode(map[string]string{"value": fmt.Sprintf("session-%d", loginHits)})
				case r.URL.Path == "/rest/com/vmware/cis/tagging/tag" && r.Method == http.MethodGet:
					atomic.AddInt32(&listTagsHits, 1)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				default:
					http.NotFound(w, r)
				}
			})
			srv := httptest.NewServer(handler)
			srv.Config.SetKeepAlivesEnabled(false)
			server = srv
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
			Expect(atomic.LoadInt32(&listTagsHits)).To(Equal(int32(1)))
			// Only the initial Login from createCollector — no re-auth.
			Expect(atomic.LoadInt32(&loginHits)).To(Equal(int32(1)))
		})
	})

	Describe("functional correctness", func() {
		It("maps tags to correct VM IDs", func() {
			server = newMockVCenter(6, "")
			collector = createCollector(server.URL)

			result, err := collector.getVMsWithTags(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeEmpty())

			for vmID, vmTags := range result {
				Expect(vmID).To(HavePrefix("vm-"))
				for _, tag := range vmTags {
					Expect(tag.Name).To(HavePrefix("name-urn:"))
					Expect(tag.ID).To(HavePrefix("urn:vmomi:"))
					Expect(tag.CategoryID).To(HavePrefix("urn:vmomi:InventoryServiceCategory:"))
				}
			}
		})
	})
})

// Ensure model.Tag is properly referenced
var _ = model.Tag{}
