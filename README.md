![CI](https://github.com/konveyor/forklift-controller/workflows/CI/badge.svg)&nbsp;[![Code Coverage](https://codecov.io/gh/konveyor/forklift-controller/branch/master/graph/badge.svg)](https://codecov.io/gh/konveyor/forklift-controller)

# forklift-controller
Konveyor Forklift controller.

---
**Logging**

Logging can be configured using environment variables:
- LOG_DEVELOPMENT: Development mode with human readable logs and (default) verbosity=4.
- LOG_LEVEL: Set the verbosity.

Verbosity:
- Info(0) used for `Info` logging.
  - Reconcile begin,end,error.
  - Condition added,update,deleted.
  - Plan postponed.
  - Migration (k8s) resources created,deleted.
  - Migration started,stopped,run (with phase),canceled,succeeded,failed.
  - Snapshot created,updated,deleted,changed.
  - Inventory watch ensured.
  - Policy agent disabled.
- Info(1) used for `Info+` logging.
  - Connection testing.
  - Plan postpone detials.
  - Pending migration details.
  - Migration (k8s) resources updated.
  - Scheduler details.
- Info(2) used for `Info++` logging.
  - Full conditions list.
  - Migrating VM status (full definition).
  - Provider inventory data reconciler started,stopped.
- Info(3) used for `Info+++` logging.
  - Inventory watch: resources changed;queued reconcile events.
  - Data reconciler: models created,updated,deleted.
  - VM validation succeeded.
- Info(4) used for `Debug` logging.
  - Policy agent HTTP request.
