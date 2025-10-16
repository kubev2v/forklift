package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/kubev2v/forklift/cmd/ova-proxy/settings"
	"github.com/kubev2v/forklift/pkg/apis"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var Settings = &settings.Settings

const (
	ProviderRoute   = "/:namespace/:provider/appliances"
	ServiceTemplate = "http://%s.%s.svc.cluster.local:8080/appliances"
)

type ProxyServer struct {
	// The service port.
	Port int
	//
	// TLS.
	TLS struct {
		// Enabled.
		Enabled bool
		// Certificate path.
		Certificate string
		// Key path
		Key string
	}
	Transport http.RoundTripper
	Client    k8sclient.Client
	Log       logr.Logger
	Cache     *ProxyCache
}

func (r *ProxyServer) Run() (err error) {
	err = r.init()
	if err != nil {
		return
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Any(ProviderRoute, r.Proxy)
	if r.TLS.Enabled {
		err = router.RunTLS(r.address(), r.TLS.Certificate, r.TLS.Key)
		if err != nil {
			r.Log.Error(err, "failed to start with TLS")
			return
		}
	} else {
		err = router.Run(r.address())
		if err != nil {
			r.Log.Error(err, "failed to start")
			return
		}
	}
	return
}

func (r *ProxyServer) Proxy(ctx *gin.Context) {
	providerName := ctx.Param("provider")
	providerNamespace := ctx.Param("namespace")

	key := path.Join(providerNamespace, providerName)
	proxy, ok := r.Cache.Get(key)
	if !ok {
		provider := &api.Provider{}
		err := r.Client.Get(ctx.Request.Context(), types.NamespacedName{
			Namespace: providerNamespace,
			Name:      providerName,
		},
			provider)
		if err != nil {
			r.Log.Error(err, "error getting provider", "provider", key)
			errorCode := http.StatusInternalServerError
			if k8serr.IsNotFound(err) {
				errorCode = http.StatusNotFound
			}
			_ = ctx.AbortWithError(errorCode, err)
			return
		}
		if provider.Status.Service == nil {
			r.Log.Error(errors.New("not ready"), "provider service is not ready")
			_ = ctx.AbortWithError(http.StatusServiceUnavailable, fmt.Errorf("provider %s service is not ready", key))
			return
		}
		service := provider.Status.Service
		svcURL := fmt.Sprintf(ServiceTemplate, service.Name, service.Namespace)
		u, err := url.Parse(svcURL)
		if err != nil {
			_ = ctx.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		proxy = &httputil.ReverseProxy{
			Rewrite: func(req *httputil.ProxyRequest) {
				req.SetURL(u)
				req.Out.URL.Path = u.Path
				req.Out.URL.RawPath = u.Path
				req.SetXForwarded()
			},
		}
		if r.Transport != nil {
			proxy.Transport = r.Transport
		}
		r.Cache.Add(key, proxy)
	}

	proxy.ServeHTTP(ctx.Writer, ctx.Request)
}

func (r *ProxyServer) init() (err error) {
	logger := logging.Factory.New()
	logf.SetLogger(logger)
	r.Log = logf.Log.WithName("entrypoint")
	r.Client, err = r.getClient()
	if err != nil {
		return
	}
	return
}

// Determine the address.
func (r *ProxyServer) address() string {
	if r.Port == 0 {
		if r.TLS.Enabled {
			r.Port = 8443
		} else {
			r.Port = 8080
		}
	}

	return fmt.Sprintf(":%d", r.Port)
}

func (r *ProxyServer) getClient() (client k8sclient.Client, err error) {
	err = apis.AddToScheme(scheme.Scheme)
	if err != nil {
		return
	}
	cfg, err := config.GetConfig()
	if err != nil {
		return
	}
	client, err = k8sclient.New(
		cfg,
		k8sclient.Options{
			Scheme: scheme.Scheme,
		})
	return
}

func main() {
	Settings.Load()
	proxy := ProxyServer{
		Cache: NewProxyCache(Settings.Cache.TTL),
	}
	if Settings.TLS.Key != "" {
		proxy.TLS.Enabled = true
		proxy.TLS.Certificate = Settings.TLS.Certificate
		proxy.TLS.Key = Settings.TLS.Key
	}
	err := proxy.Run()
	if err != nil {
		panic(err)
	}
}
