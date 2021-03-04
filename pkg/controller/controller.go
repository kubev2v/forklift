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

package controller

import (
	"github.com/konveyor/forklift-controller/pkg/controller/hook"
	"github.com/konveyor/forklift-controller/pkg/controller/host"
	"github.com/konveyor/forklift-controller/pkg/controller/map/network"
	"github.com/konveyor/forklift-controller/pkg/controller/map/storage"
	"github.com/konveyor/forklift-controller/pkg/controller/migration"
	"github.com/konveyor/forklift-controller/pkg/controller/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/provider"
	"github.com/konveyor/forklift-controller/pkg/settings"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

//
// Application settings.
var Settings = &settings.Settings

//
// Function provided by controller packages to add
// them self to the manager.
type AddFunction func(manager.Manager) error

//
// List of main controllers
var MainControllers = []AddFunction{
	migration.Add,
	plan.Add,
	network.Add,
	storage.Add,
	host.Add,
	hook.Add,
}

//
// List of Inventory controllers
var InventoryControllers = []AddFunction{
	provider.Add,
}

//
// Add controllers to the manager based on role.
func AddToManager(m manager.Manager) error {
	load := func(functions []AddFunction) error {
		for _, f := range functions {
			if err := f(m); err != nil {
				return err
			}
		}
		return nil
	}
	if Settings.Role.Has(settings.InventoryRole) {
		err := load(InventoryControllers)
		if err != nil {
			return err
		}

	}
	if Settings.Role.Has(settings.MainRole) {
		err := load(MainControllers)
		if err != nil {
			return err
		}

	}

	return nil
}
