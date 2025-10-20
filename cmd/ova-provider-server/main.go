package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/api"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/auth"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/settings"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var Settings = &settings.Settings
var log = logging.WithName("ova|main")

func main() {
	var err error
	defer func() {
		if err != nil {
			log.Error(err, "router returned error")
		}
	}()

	err = Settings.Load()
	if err != nil {
		log.Error(err, "failed to load settings")
		panic(err)
	}
	log.Info("Started", "settings", Settings)

	router := gin.Default()
	router.Use(api.ErrorHandler())

	inventory := api.InventoryHandler{}
	inventory.AddRoutes(router)
	if Settings.ApplianceEndpoints {
		appliances := api.ApplianceHandler{
			OVAStoragePath: Settings.CatalogPath,
			AuthRequired:   Settings.Auth.Required,
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
