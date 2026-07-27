// storage-layer-populator-simulator simulates a storage-layer operator (e.g.
// Portworx Stork, IBM Spectrum) by watching PortworxXcopyVolumePopulator CRs
// and performing a real CSI volume clone from the source (FADA) PVC into the
// destination PVC. The destination PVC ends up Bound with the cloned data.
//
// This binary exists only for testing the two-phase migration flow without a
// real storage-layer operator. Delete the entire cmd/storage-layer-populator-simulator/
// directory when the real operator integration is ready.
package main

import (
	"flag"
	"os"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	_ = core.AddToScheme(scheme)
	_ = forkliftv1beta1.SchemeBuilder.AddToScheme(scheme)

	logger := logging.Factory.New()
	logf.SetLogger(logger)
}

func main() {
	var metricsAddr string
	var namespace string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8083", "The address the metric endpoint binds to.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to watch. Empty means all namespaces.")
	flag.Parse()

	log := logf.Log.WithName("storage-layer-populator-simulator")

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&PopulatorReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Log:       log,
		Namespace: namespace,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to set up PortworxXcopyVolumePopulator reconciler")
		os.Exit(1)
	}

	log.Info("Starting storage-layer-populator-simulator")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}
