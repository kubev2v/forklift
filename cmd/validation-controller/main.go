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

package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	validationv1alpha1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	controller "github.com/kubev2v/forklift/pkg/validation/controller"
	validationmetrics "github.com/kubev2v/forklift/pkg/validation/metrics"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	controllermetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	logger := log.New(os.Stdout, "validation-controller: ", log.LstdFlags|log.LUTC)
	// controller-runtime uses logr internally. Configure it before creating the
	// manager so leader election and reconciliation do not fall back to the
	// no-op logger (which also emits a warning on the first log call).
	ctrl.SetLogger(logr.FromSlogHandler(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	var validatorImage string
	var leaderElectionID string
	var leaderElectionNamespace string
	var metricsClientCN string
	var leaderElect bool
	flag.StringVar(&validatorImage, "validator-image", os.Getenv("VALIDATOR_IMAGE"), "validator image to run")
	flag.BoolVar(&leaderElect, "leader-elect", true, "enable leader election when more than one controller replica is configured")
	flag.StringVar(&leaderElectionID, "leader-election-id", "forklift-validation-controller.forklift.konveyor.io", "leader election lease name")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", os.Getenv("POD_NAMESPACE"), "namespace containing the leader election lease")
	flag.StringVar(&metricsClientCN, "metrics-client-cn", validationmetrics.DefaultPrometheusClientCN, "mTLS client certificate common name allowed to scrape metrics")
	flag.Parse()
	if validatorImage == "" {
		logger.Printf("validator image is required (set --validator-image or VALIDATOR_IMAGE to an immutable image reference)")
		os.Exit(2)
	}
	if leaderElectionNamespace == "" {
		logger.Printf("leader election namespace is required (set --leader-election-namespace or POD_NAMESPACE)")
		os.Exit(2)
	}

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = coordinationv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = validationv1alpha1.SchemeBuilder.AddToScheme(scheme)
	validationLabel, err := labels.NewRequirement("forklift.konveyor.io/validation", selection.Exists, nil)
	if err != nil {
		logger.Printf("unable to build validation resource selector: %v", err)
		os.Exit(1)
	}
	validationResources := labels.NewSelector()
	validationResources = validationResources.Add(*validationLabel)
	context := ctrl.SetupSignalHandler()
	configuration := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(configuration)
	if err != nil {
		logger.Printf("unable to create metrics client: %v", err)
		os.Exit(1)
	}
	clientCAs, err := validationmetrics.StartClientCAWatcher(context, clientset, ctrl.Log.WithName("metrics"))
	if err != nil {
		logger.Printf("unable to configure metrics mTLS: %v", err)
		os.Exit(1)
	}
	mgr, err := ctrl.NewManager(configuration, ctrl.Options{
		Scheme:                  scheme,
		HealthProbeBindAddress:  ":8081",
		LeaderElection:          leaderElect,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNamespace,
		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&batchv1.Job{}:      {Label: validationResources},
			&corev1.ConfigMap{}: {Label: validationResources},
		}},
		Metrics: metricsserver.Options{SecureServing: true, BindAddress: ":8443", CertDir: "/etc/tls/private", TLSOpts: validationmetrics.TLSOptions(clientCAs), FilterProvider: validationmetrics.AllowClientCN(metricsClientCN)},
	})
	if err != nil {
		logger.Printf("unable to create controller manager: %v", err)
		os.Exit(1)
	}
	validationMetrics, err := validationmetrics.NewValidationMetrics(mgr.GetClient(), controllermetrics.Registry)
	if err != nil {
		logger.Printf("unable to register validation metrics: %v", err)
		os.Exit(1)
	}
	if err := (&controller.VirtualizationValidationReconciler{Client: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Scheme: mgr.GetScheme(), ValidatorImage: validatorImage, Recorder: mgr.GetEventRecorderFor("virt-validation-controller"), Metrics: validationMetrics}).SetupWithManager(mgr); err != nil {
		logger.Printf("unable to set up VirtualizationValidation controller: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Printf("unable to add health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Printf("unable to add readiness check: %v", err)
		os.Exit(1)
	}
	logger.Printf("starting manager: watchScope=cluster validatorImage=%q leaderElection=%t leaderElectionID=%q leaderElectionNamespace=%q", validatorImage, leaderElect, leaderElectionID, leaderElectionNamespace)
	go func() {
		<-mgr.Elected()
		logger.Printf("leader election acquired; reconciliation is enabled")
	}()
	if err := mgr.Start(context); err != nil {
		logger.Printf("controller manager stopped with an error: %v", err)
		os.Exit(1)
	}
	logger.Printf("controller manager stopped")
}
