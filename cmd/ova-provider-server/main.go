package main

import (
	"context"
	"log"
	"net/http"

	"github.com/konveyor/forklift-controller/cmd/ova-provider-server/catalog"
	"github.com/konveyor/forklift-controller/cmd/ova-provider-server/settings"
)

var Settings = &settings.Settings

func main() {
	vmIDMap = NewUUIDMap()
	diskIDMap = NewUUIDMap()
	networkIDMap = NewUUIDMap()

	err := Settings.Load()
	if err != nil {
		log.Fatal(err)
	}
	manager, err := catalog.New(
		Settings.CatalogPath,
		Settings.ConfigPath,
		Settings.ScanInterval,
		Settings.Prune,
		Settings.MaxConcurrentDownloads,
	)
	if err != nil {
		log.Fatal(err)
	}
	err = manager.Run(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/vms", vmHandler)
	http.HandleFunc("/disks", diskHandler)
	http.HandleFunc("/networks", networkHandler)
	http.HandleFunc("/watch", watchdHandler)
	http.HandleFunc("/test_connection", connHandler)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
