package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/provider-common/api"
	"github.com/kubev2v/forklift/cmd/provider-common/auth"
	"github.com/kubev2v/forklift/cmd/provider-common/inventory"
	"github.com/kubev2v/forklift/cmd/provider-common/ovf"
	"github.com/kubev2v/forklift/cmd/provider-common/settings"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var Settings = &settings.ProviderSettings{
	DefaultCatalogPath: "/ova",
}

var log = logging.WithName("ova|main")

func main() {
	var err error
	defer func() {
		if err != nil {
			log.Error(err, "router returned error")
		}
	}()

	// Set the logger name for the API package
	api.SetLogger("ova|api")

	err = Settings.Load()
	if err != nil {
		log.Error(err, "failed to load settings")
		panic(err)
	}
	log.Info("Started", "settings", Settings)

	router := gin.Default()
	router.Use(api.ErrorHandler())

	inventoryHandler := api.InventoryHandler{
		Settings:     Settings,
		ProviderType: inventory.ProviderTypeOVA,
	}
	inventoryHandler.AddRoutes(router)

	if Settings.ApplianceEndpoints {
		appliances := api.ApplianceHandler{
			StoragePath:   Settings.CatalogPath,
			AuthRequired:  Settings.Auth.Required,
			FileExtension: ovf.ExtOVA,
			Auth: auth.NewProviderAuth(
				Settings.Provider.Namespace,
				Settings.Provider.Name,
				Settings.Provider.Verb,
				Settings.Auth.TTL),
		}
		appliances.AddRoutes(router)
	}

	err = router.Run(fmt.Sprintf(":%s", Settings.Port))
}
