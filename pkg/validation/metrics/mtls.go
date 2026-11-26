// Copyright (C) 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package metrics configures the OpenShift mTLS metrics endpoint.
package metrics

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	clientCAConfigMapNamespace = "kube-system"
	clientCAConfigMapName      = "extension-apiserver-authentication"
	clientCAConfigMapKey       = "client-ca-file"
	// DefaultPrometheusClientCN is the platform-monitoring identity issued by
	// OpenShift's metrics client CA.
	DefaultPrometheusClientCN = "system:serviceaccount:openshift-monitoring:prometheus-k8s"
)

// ClientCAPool supplies the current OpenShift metrics client CA to TLS.
type ClientCAPool struct{ pool atomic.Pointer[x509.CertPool] }

func (p *ClientCAPool) set(pem []byte) error {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return fmt.Errorf("no valid certificates in metrics client CA")
	}
	p.pool.Store(pool)
	return nil
}

func (p *ClientCAPool) get() *x509.CertPool { return p.pool.Load() }

// StartClientCAWatcher uses OpenShift's authoritative client CA and follows
// rotations without restarting the controller.
func StartClientCAWatcher(ctx context.Context, client kubernetes.Interface, logger logr.Logger) (*ClientCAPool, error) {
	configMap, err := client.CoreV1().ConfigMaps(clientCAConfigMapNamespace).Get(ctx, clientCAConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get metrics client CA: %w", err)
	}
	pool := &ClientCAPool{}
	if err := updateClientCA(pool, configMap); err != nil {
		return nil, err
	}
	informer := coreinformers.NewFilteredConfigMapInformer(client, clientCAConfigMapNamespace, 0, cache.Indexers{}, func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("metadata.name", clientCAConfigMapName).String()
	})
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(object interface{}) { updateClientCAObject(pool, object, logger) }, UpdateFunc: func(_, object interface{}) { updateClientCAObject(pool, object, logger) }}); err != nil {
		return nil, fmt.Errorf("watch metrics client CA: %w", err)
	}
	go informer.Run(ctx.Done())
	return pool, nil
}

func updateClientCA(pool *ClientCAPool, configMap *corev1.ConfigMap) error {
	return pool.set([]byte(configMap.Data[clientCAConfigMapKey]))
}

func updateClientCAObject(pool *ClientCAPool, object interface{}, logger logr.Logger) {
	if configMap, ok := object.(*corev1.ConfigMap); ok {
		if err := updateClientCA(pool, configMap); err != nil {
			logger.Error(err, "ignoring invalid metrics client CA update; retaining the previous CA")
		}
	}
}

// TLSOptions requires and verifies the Prometheus mTLS client certificate.
func TLSOptions(pool *ClientCAPool) []func(*tls.Config) {
	return []func(*tls.Config){func(config *tls.Config) {
		config.MinVersion = tls.VersionTLS12
		config.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
			clientCAs := pool.get()
			if clientCAs == nil {
				return nil, fmt.Errorf("metrics client CA is unavailable")
			}
			tlsConfig := config.Clone()
			tlsConfig.GetConfigForClient = nil
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			tlsConfig.ClientCAs = clientCAs
			return tlsConfig, nil
		}
	}}
}

// AllowClientCN admits only the requested metrics client identity.
func AllowClientCN(allowedCN string) func(*rest.Config, *http.Client) (metricsserver.Filter, error) {
	return func(_ *rest.Config, _ *http.Client) (metricsserver.Filter, error) {
		return allowClientCN(allowedCN), nil
	}
}

func allowClientCN(allowedCN string) metricsserver.Filter {
	return func(_ logr.Logger, next http.Handler) (http.Handler, error) {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.TLS == nil || len(request.TLS.PeerCertificates) == 0 {
				http.Error(writer, "client certificate required", http.StatusUnauthorized)
				return
			}
			if request.TLS.PeerCertificates[0].Subject.CommonName != allowedCN {
				http.Error(writer, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(writer, request)
		}), nil
	}
}

var _ metricsserver.Filter = allowClientCN(DefaultPrometheusClientCN)
