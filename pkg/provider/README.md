## Provider Package

This directory contains Forklift provider integrations. Each subdirectory under `pkg/provider/<name>` encapsulates everything needed to discover inventory from a source platform and drive VM migrations for that platform.

To add a provider, use an existing one (for example, `ec2` or `azure`) as a reference and complete the steps below.

## Adding a New Provider

### 1. Define the provider type

Add a constant and `ProviderTypes` entry in `pkg/apis/forklift/v1beta1/provider.go`.

### 2. Register factory switch-cases

Each registration is a `case api.<Type>:` in the relevant `doc.go` or factory file:

| Registration point | File |
|--------------------|------|
| Inventory collector | `pkg/controller/provider/container/doc.go` |
| Inventory model | `pkg/controller/provider/model/doc.go` |
| Web handlers | `pkg/controller/provider/web/doc.go` |
| Web client (Finder + Resolver) | `pkg/controller/provider/web/client.go` |
| Plan adapter | `pkg/controller/plan/adapter/doc.go` |
| Plan event handler | `pkg/controller/plan/handler/doc.go` |
| Migrator | `pkg/controller/plan/migrator/doc.go` |
| Scheduler | `pkg/controller/plan/scheduler/doc.go` |
| Host handler | `pkg/controller/host/handler/doc.go` |
| Network map handler | `pkg/controller/map/network/handler/doc.go` |
| Storage map handler | `pkg/controller/map/storage/handler/doc.go` |

### 3. Implement the provider

Create the provider under `pkg/provider/<name>` following the common layout described below.

## Common Provider Layout

```
pkg/provider/<name>/
├── constants.go            # Annotations, labels, tags (single source of truth)
├── README.md               # Migration flow, credentials, requirements
├── auth/                   # Shared credential extraction (recommended)
├── docs/                   # Design docs (architecture decisions, feature comparisons)
├── testutil/               # Fake API implementations and test fixtures
├── inventory/
│   ├── client/             # Platform SDK wrapper + API interface for test injection
│   ├── collector/          # Polling-based inventory collector
│   ├── model/              # SQLite DB models
│   └── web/                # REST API handlers, Finder, Resolver
└── controller/
    ├── adapter/            # Adapter factory (wires Builder, Client, Validator, Ensurer)
    ├── builder/            # KubeVirt VM spec builder + volume/PVC builders
    ├── client/             # Migration client (power ops, snapshots, transfers)
    ├── ensurer/            # Kubernetes resource ensurer (idempotent create)
    ├── handler/            # Plan, NetworkMap, StorageMap event handlers
    ├── inventory/          # Controller-side inventory helpers
    ├── mapping/            # Storage/network mapping lookup utilities
    ├── migrator/           # Migration itinerary, phase executor, pipeline
    ├── scheduler/          # MaxInFlight concurrency limiter
    └── validator/          # Pre-migration validation
```

Not every directory is required -- omit packages that don't apply (e.g., `auth/` if credentials are trivial, `docs/` if the design is straightforward).

## Best Practices

- **Centralize constants** -- Keep all annotation keys, label keys, and cloud resource tags in a root-level `constants.go` rather than scattering them across packages.
- **Share credentials** -- When both inventory and controller clients need platform credentials, extract parsing and SDK credential creation into an `auth/` package.
- **Define API interfaces** -- Create a `<platform>api.go` interface file in each client package so unit tests can inject fakes without calling real cloud APIs.
- **Provide testutil/** -- Include fake API implementations and fixture builders. This makes it straightforward to test builders, validators, and migrator phases in isolation.
- **Use `noop.go` files** -- Satisfy interface methods that don't apply to your provider with explicit no-ops rather than embedding logic in unrelated files.
- **Document the provider** -- Include a `README.md` covering migration flow, credential format, RBAC requirements, and provider-specific settings.
- **Keep design docs in `docs/`** -- Architecture decisions (disk transfer strategy, feature comparison with other providers, tagging conventions) belong alongside the code.
