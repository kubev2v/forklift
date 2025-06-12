/*
Copyright 2019 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package forklift_api

import (
	"net/http"
	"os"

	"github.com/kubev2v/forklift/pkg/forklift-api/services"
	webhooks "github.com/kubev2v/forklift/pkg/forklift-api/webhooks"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Default port that forklift-api listens on.
	defaultPort = 443

	// Default address that forklift-api listens on.
	defaultHost = "0.0.0.0"
)

var log = logging.WithName("forklift-api")

type ForkliftApi interface {
	Execute()
}

type forkliftAPIApp struct {
	Name        string
	BindAddress string
	Port        int
	client      client.Client
}

func NewForkliftApi(client client.Client) ForkliftApi {

	app := &forkliftAPIApp{}
	app.BindAddress = defaultHost
	app.Port = defaultPort
	app.client = client

	return app
}

func (app *forkliftAPIApp) Execute() {
	go app.serveServices()
	app.serveWebhooks()
}

func (app *forkliftAPIApp) serveServices() {
	servicesTlsCertificate, found := os.LookupEnv("SERVICES_TLS_CERTIFICATE")
	if !found {
		log.Info("Failed to find SERVICES_TLS_CERTIFICATE")
		return
	}
	servicesTlsKey, found := os.LookupEnv("SERVICES_TLS_KEY")
	if !found {
		log.Info("Failed to find SERVICES_TLS_KEY")
		return
	}

	mux := http.NewServeMux()
	services.RegisterServices(mux, app.client)
	server := http.Server{
		Addr:    ":8444",
		Handler: mux,
	}

	log.Info("start serving services")
	err := server.ListenAndServeTLS(servicesTlsCertificate, servicesTlsKey)
	if err != nil {
		log.Error(err, "stop serving services")
	}
}

func (app *forkliftAPIApp) serveWebhooks() {
	apiTlsCertificate, found := os.LookupEnv("API_TLS_CERTIFICATE")
	if !found {
		log.Info("Failed to find API_TLS_CERTIFICATE")
		return
	}
	apiTlsKey, found := os.LookupEnv("API_TLS_KEY")
	if !found {
		log.Info("Failed to find API_TLS_KEY")
		return
	}

	mux := http.NewServeMux()
	webhooks.RegisterMutatingWebhooks(mux, app.client)
	webhooks.RegisterValidatingWebhooks(mux, app.client)
	server := http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	log.Info("start serving webhooks")
	err := server.ListenAndServeTLS(apiTlsCertificate, apiTlsKey)
	if err != nil {
		log.Error(err, "stop serving webhooks")
	}
}
