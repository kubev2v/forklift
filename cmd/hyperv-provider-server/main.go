package main

import (
	"fmt"

	"github.com/kubev2v/forklift/cmd/hyperv-provider-server/collector"
	"github.com/kubev2v/forklift/cmd/hyperv-provider-server/handler"
	"github.com/kubev2v/forklift/cmd/provider-common/api"
	"github.com/kubev2v/forklift/cmd/provider-common/settings"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var Settings = &settings.ProviderSettings{
	DefaultCatalogPath: "/hyperv",
}

var log = logging.WithName("hyperv|main")

func main() {
	var err error
	defer func() {
		if err != nil {
			log.Error(err, "router returned error")
		}
	}()

	// Set the logger name for the API package
	api.SetLogger("hyperv|api")

	// Load common settings
	err = Settings.Load()
	if err != nil {
		log.Error(err, "failed to load settings")
		panic(err)
	}

	router := logging.GinEngine()
	// Load HyperV-specific settings
	err = Settings.LoadHyperV()
	if err != nil {
		log.Error(err, "failed to load HyperV settings")
		panic(err)
	}

	log.Info("Started",
		"provider", Settings.Provider.Name,
		"namespace", Settings.Provider.Namespace,
		"hypervUrl", Settings.HyperV.URL,
		"smbUrl", Settings.HyperV.SMBUrl,
		"smbMountPath", Settings.CatalogPath,
		"refreshInterval", Settings.HyperV.RefreshInterval)

	hvCollector := collector.NewCollector(Settings)
	go hvCollector.Start()

	router.Use(api.ErrorHandler())

	inventoryHandler := handler.InventoryHandler{
		Collector: hvCollector,
	}
	inventoryHandler.AddRoutes(router)

	log.Info("Starting HTTP server", "port", Settings.Port)
	err = router.Run(fmt.Sprintf(":%s", Settings.Port))
}
