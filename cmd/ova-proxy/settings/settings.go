package settings

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// k8s pod default.
const (
	ServiceCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
)

// DefaultScheme is the default scheme for the inventory service.
const (
	DefaultScheme = "https"
)

// Environment variables.
const (
	Port           = "PORT"
	Namespace      = "POD_NAMESPACE"
	Scheme         = "SERVICE_SCHEME"
	TLSCertificate = "TLS_CERTIFICATE"
	TLSKey         = "TLS_KEY"
	TLSCa          = "TLS_CA"
	CacheTTL       = "CACHE_TTL"
)

var Settings = ProxySettings{}

type ProxySettings struct {
	// Pod namespace
	Namespace string
	// Host.
	Host string
	// Port
	Port int
	// URL Scheme (http or https)
	Scheme string
	// TLS
	TLS struct {
		// Certificate path
		Certificate string
		// Key path
		Key string
		// CA path
		CA string
	}
	// Cache
	Cache struct {
		// TTL (in seconds)
		TTL int64
	}
}

func (r *ProxySettings) Load() {
	// Namespace
	if s, found := os.LookupEnv(Namespace); found {
		r.Namespace = s
	}
	// Port
	if s, found := os.LookupEnv(Port); found {
		r.Port, _ = strconv.Atoi(s)
	} else {
		r.Port = 8080
	}
	// Scheme
	if s, found := os.LookupEnv(Scheme); found {
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "http", "https":
			r.Scheme = s
		default:
			r.Scheme = DefaultScheme
		}
	} else {
		r.Scheme = DefaultScheme
	}
	// TLS
	if s, found := os.LookupEnv(TLSCertificate); found {
		r.TLS.Certificate = s
	}
	if s, found := os.LookupEnv(TLSKey); found {
		r.TLS.Key = s
	}
	if s, found := os.LookupEnv(TLSCa); found {
		r.TLS.CA = s
	} else {
		if _, err := os.Stat(ServiceCAFile); !errors.Is(err, os.ErrNotExist) {
			r.TLS.CA = ServiceCAFile
		}
	}
	if s, found := os.LookupEnv(CacheTTL); found {
		r.Cache.TTL, _ = strconv.ParseInt(s, 10, 64)
	} else {
		r.Cache.TTL = 10 // seconds
	}
}
