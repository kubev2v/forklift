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

package forklift_api

import (
	webhooks "github.com/konveyor/forklift-controller/pkg/forklift-api/webhooks"
)

const (
        // Default port that virt-api listens on.
        defaultPort = 443

        // Default address that virt-api listens on.
        defaultHost = "0.0.0.0"
)

type ForkliftApi interface {
        Execute()
}

type forkliftAPIApp struct {
        Name        string
        BindAddress string
        Port        int
}

func NewForkliftApi() ForkliftApi {

        app := &forkliftAPIApp{}
        app.BindAddress = defaultHost
        app.Port = defaultPort

        return app
}

func (app *forkliftAPIApp) Execute() {
	webhooks.RegisterValidatingWebhooks()
}
