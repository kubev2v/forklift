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
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/kubev2v/forklift/pkg/apis"
	forklift_api "github.com/kubev2v/forklift/pkg/forklift-api"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "unable to add forklift API to scheme")
		os.Exit(1)
	}

	err = api.SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't build the scheme")
		os.Exit(1)
	}

	err = net.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Error(err, "Couldn't add network-attachment-definition-client to the scheme")
		os.Exit(1)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Couldn't get the cluster configuration")
		os.Exit(1)
	}

	client, err := client.New(config, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "Couldn't create a cluster client")
		os.Exit(1)
	}

	app := forklift_api.NewForkliftApi(client)
	app.Execute()
}
