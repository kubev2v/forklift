package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/catalog"
	"github.com/kubev2v/forklift/cmd/ova-provider-server/settings"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var Settings = &settings.Settings
var log = logging.WithName("ova")

func main() {
	log.Info("Started", "settings", Settings)

	vmIDMap = NewUUIDMap()
	diskIDMap = NewUUIDMap()
	networkIDMap = NewUUIDMap()

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
	manager, err := catalog.New(
		Settings.CatalogPath,
		Settings.SourcesPath,
		Settings.ScanInterval,
		Settings.Prune,
		Settings.MaxConcurrentDownloads,
		Settings.DownloadTimeout,
	)
	if err != nil {
		log.Error(err, "failed to create catalog manager")
		return
	}
	err = manager.Run(context.Background())
	if err != nil {
		log.Error(err, "failed while running catalog manager")
		return
	}

	router := gin.Default()
	router.Use(ErrorHandler())
	router.GET("/vms", gin.WrapF(vmHandler))
	router.GET("/disks", gin.WrapF(diskHandler))
	router.GET("/networks", gin.WrapF(networkHandler))
	router.GET("/watch", gin.WrapF(watchdHandler))
	router.GET("/test_connection", gin.WrapF(connHandler))

	handler := catalog.Handler{Manager: manager}
	handler.AddRoutes(router)

	err = router.Run(fmt.Sprintf(":%s", Settings.Port))
}

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		if len(ctx.Errors) == 0 {
			return
		}
		err := ctx.Errors[0]
		switch {
		default:
			ctx.JSON(http.StatusBadRequest,
				gin.H{"error": err.Error()})
		}
	}
}
