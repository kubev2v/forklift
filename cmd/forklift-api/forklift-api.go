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
	"os"

	"github.com/go-logr/logr"
	"github.com/konveyor/forklift-controller/pkg/apis"
	forklift_api "github.com/konveyor/forklift-controller/pkg/forklift-api"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"k8s.io/client-go/kubernetes/scheme"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log logr.Logger

func init() {
	logger := logging.Factory.New()
	logf.SetLogger(logger)
	log = logf.Log.WithName("entrypoint")
}

func main() {
	log.Info("start forklift-api")
	app := forklift_api.NewForkliftApi()

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "unable to add forklift API to scheme")
		os.Exit(1)
	}

	app.Execute()
}
