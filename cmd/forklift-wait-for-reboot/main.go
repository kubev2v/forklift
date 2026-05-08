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

// forklift-wait-for-reboot watches a KubeVirt VMI serial console for a conversion
// signal, then polls VMI phase to detect a guest reboot.
package main

import (
	"context"
	"log"
	"os"

	waitforreboot "github.com/kubev2v/forklift/pkg/wait-for-reboot"
	"k8s.io/client-go/rest"
)

func main() {
	log.SetPrefix("forklift-wait-for-reboot: ")
	log.SetFlags(log.LstdFlags)

	cfg, err := waitforreboot.ParseConfig()
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("in-cluster config: %v", err)
	}

	code := waitforreboot.Watch(context.Background(), restCfg, log.Default(), cfg)
	os.Exit(code)
}
