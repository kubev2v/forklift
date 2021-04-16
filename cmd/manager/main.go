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

package main

import (
	"github.com/konveyor/controller/pkg/logging"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"os"
	"strconv"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/konveyor/forklift-controller/pkg/apis"
	"github.com/konveyor/forklift-controller/pkg/controller"
	"github.com/konveyor/forklift-controller/pkg/webhook"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	cnv "kubevirt.io/client-go/api/v1"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"

	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

//
// Application settings.
var Settings = &settings.Settings

func main() {
	development := false
	if s, found := os.LookupEnv(logging.EnvDevelopment); found {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			development = parsed
		}
	}

	logf.SetLogger(logf.ZapLogger(development))

	log := logf.Log.WithName("entrypoint")

	// Load settings.
	err := Settings.Load()
	if err != nil {
		log.Error(err, "unable to load settings")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	log.Info("setting up client for manager")
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to set up client config")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	log.Info("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: Settings.Metrics.Address(),
	})
	if err != nil {
		log.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	log.Info("setting up scheme")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add K8s APIs to scheme")
		os.Exit(1)
	}
	if err := net.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add CNI APIs to scheme")
		os.Exit(1)
	}
	if err := cnv.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add kubevirt APIs to scheme")
		os.Exit(1)
	}
	if err := vmio.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add kubevirt VMIO APIs to scheme")
		os.Exit(1)
	}
	if err := cdi.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "unable to add kubevirt CDI APIs to scheme")
		os.Exit(1)
	}
	// Setup all Controllers
	log.Info("Setting up controller")
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "unable to register controllers to the manager")
		os.Exit(1)
	}
	log.Info("setting up webhooks")
	if err := webhook.AddToManager(mgr); err != nil {
		log.Error(err, "unable to register webhooks to the manager")
		os.Exit(1)
	}
	// Start the Cmd
	log.Info("Starting the Cmd.")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "unable to run the manager")
		os.Exit(1)
	}
}
