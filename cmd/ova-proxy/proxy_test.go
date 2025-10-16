package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/kubev2v/forklift/pkg/apis"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOVAProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OVA Proxy Suite")
}

func init() {
	gin.SetMode(gin.TestMode)
}

var _ = Describe("ProxyServer", func() {
	Describe("address()", func() {
		DescribeTable("defaults and explicit",
			func(port int, tlsOn bool, expect string) {
				ps := &ProxyServer{Port: port}
				ps.TLS.Enabled = tlsOn
				Expect(ps.address()).To(Equal(expect))
			},
			Entry("nonTLS default", 0, false, ":8080"),
			Entry("TLS default", 0, true, ":8443"),
			Entry("explicit", 9000, false, ":9000"),
		)
	})

	Describe("Proxy()", func() {
		It("returns 404 when the Provider is missing", func() {
			spy := NewSpyClient()
			proxy := &ProxyServer{
				Client: spy,
				Log:    logr.Discard(),
				Cache:  NewProxyCache(300),
			}

			ns, name := "namespace", "not-found"
			urlPath := path.Join("/", ns, name, "appliances")
			ctx, recorder := makeCtx(http.MethodGet, ns, name, urlPath)

			proxy.Proxy(ctx)

			Expect(recorder.Code).To(Equal(http.StatusNotFound), "body: %q", recorder.Body.String())
			Expect(spy.gets).To(Equal(1))
			Expect(spy.lastKey).To(Equal(types.NamespacedName{Namespace: ns, Name: name}))
		})

		It("returns 503 when the Provider service is not ready", func() {
			provider := NewProvider("konveyor", "provider", nil)
			spy := NewSpyClient(provider)

			srv := &ProxyServer{
				Client: spy,
				Log:    logr.Discard(),
				Cache:  NewProxyCache(300),
			}

			ctx, recorder := makeCtx(http.MethodGet, "konveyor", "provider", "/konveyor/provider/appliances")
			srv.Proxy(ctx)

			Expect(recorder.Code).To(Equal(http.StatusServiceUnavailable), "body: %q", recorder.Body.String())
			Expect(spy.gets).To(Equal(1))
		})

		It("proxies to the backend and uses the cache on subsequent calls", func() {
			// Backend that confirms it receives /appliances and replies with a known body.
			var hits int
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				if r.URL.Path != "/appliances" {
					http.Error(w, "unexpected path: "+r.URL.Path, http.StatusBadGateway)
					return
				}
				_, _ = fmt.Fprint(w, "ok")
			}))
			DeferCleanup(backend.Close)

			ns, name := "konveyor", "provider"
			provider := NewProvider(ns, name, &corev1.ObjectReference{
				Name:      "svc-name",
				Namespace: "svc-ns",
			})
			spy := NewSpyClient(provider)
			parsed, _ := url.Parse(backend.URL)
			svcDialRedirect(parsed.Host)

			proxy := &ProxyServer{
				Client: spy,
				Log:    logr.Discard(),
				Cache:  NewProxyCache(300),
			}
			// path that the proxy should resolve into a request to /appliances
			// against the server for the provider specified by the URL params
			requestPath := path.Join("/", ns, name, "appliances")

			// this request should be a cache miss
			ctx, recorder := makeCtx(http.MethodGet, ns, name, requestPath)
			proxy.Proxy(ctx)
			Expect(recorder.Code).To(Equal(http.StatusOK), "first body: %q", recorder.Body.String())
			Expect(strings.TrimSpace(recorder.Body.String())).To(Equal("ok"))
			Expect(spy.gets).To(Equal(1))
			Expect(hits).To(Equal(1))

			// this request should be a cache hit
			ctx, recorder = makeCtx(http.MethodGet, ns, name, requestPath)
			proxy.Proxy(ctx)
			Expect(recorder.Code).To(Equal(http.StatusOK), "second body: %q", recorder.Body.String())
			Expect(spy.gets).To(Equal(1), "provider should be cached")
			Expect(hits).To(Equal(2))
		})
	})
})

// SpyClient wraps a controller-runtime client to count Get() calls and record the last key.
type SpyClient struct {
	client.Client
	gets    int
	lastKey types.NamespacedName
}

func (s *SpyClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	s.gets++
	s.lastKey = key
	return s.Client.Get(ctx, key, obj, opts...)
}

func NewSpyClient(objs ...client.Object) *SpyClient {
	scheme := runtime.NewScheme()
	err := apis.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred(), "AddToScheme failed")
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &SpyClient{Client: base}
}

func NewProvider(ns, name string, svc *corev1.ObjectReference) *api.Provider {
	return &api.Provider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "forklift.konveyor.io/v1beta1",
			Kind:       "Provider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Status: api.ProviderStatus{
			Service: svc,
		},
	}
}

// closeNotifyRecorder wraps httptest.ResponseRecorder to implement http.CloseNotifier
type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
}

func (c *closeNotifyRecorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

func makeCtx(method, ns, provider, urlPath string) (*gin.Context, *closeNotifyRecorder) {
	recorder := &closeNotifyRecorder{httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(method, urlPath, nil)
	c.Request = request
	c.Params = gin.Params{
		{Key: "namespace", Value: ns},
		{Key: "provider", Value: provider},
	}
	return c, recorder
}

// svcDialRedirect redirects any dial to "*.svc.cluster.local:8080" to backendAddr for the duration
// of the current spec, and restores the transport afterward.
func svcDialRedirect(backendAddr string) {
	original := http.DefaultTransport.(*http.Transport)
	clone := original.Clone()
	clone.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasSuffix(addr, ".svc.cluster.local:8080") {
			addr = backendAddr
		}
		d := &net.Dialer{Timeout: 5 * time.Second}
		return d.DialContext(ctx, network, addr)
	}
	http.DefaultTransport = clone
	DeferCleanup(func() { http.DefaultTransport = original })
}
