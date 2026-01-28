## Provider Package

This directory contains Forklift provider integrations. Each subdirectory under `pkg/provider/<name>` encapsulates everything needed to discover inventory from a source platform and drive VM migrations for that platform.

Providers follow a common structure:
- `inventory/`: Fetches and exposes source resources (VMs, networks, storage) via the Forklift API.
- `controller/`: Implements migration behavior (adapters, builders, validators, handlers, schedulers).

## Adding a New Provider

To add a provider, use an existing one (for example, `ec2`) as a reference and complete these steps:

1. **Define the provider type** in `pkg/apis/forklift/v1beta1/provider.go` (constant and `ProviderTypes` entry).
2. **Register the inventory collector** in `pkg/controller/provider/container/doc.go` so the controller can pull inventory.
3. **Register web handlers** in `pkg/controller/provider/web/doc.go` so inventory is exposed over the API.
4. **Register the plan adapter** in `pkg/controller/plan/adapter/doc.go` to implement migration logic.
5. **Register the plan event handler** in `pkg/controller/plan/handler/doc.go` to react to inventory and plan changes.
6. **Register the host handler** in `pkg/controller/host/handler/doc.go` (or a no-op handler if the provider has no host concept).

After wiring these points, implement the new provider under `pkg/provider/<name>` following the common layout.