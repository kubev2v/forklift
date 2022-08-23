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
        "github.com/go-logr/logr"
        forklift_api "github.com/konveyor/forklift-controller/pkg/forklift-api"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log logr.Logger

func init() {
        log = logf.Log.WithName("entrypoint")
}

func main() {
        log.Info("start forklift-api")
        app := forklift_api.NewForkliftApi()
        app.Execute()
}
