# Hacking on forklift-controller

## Building and running forklift-controller with `make run`

__1. Install prerequisites__

 - golang compiler (tested @ 1.11.5)
 - dep (tested @ v0.5.0)

__2. Clone the project to your `$GOPATH`__

Clone forklift-controller to your $GOPATH so that dependencies in `vendor` will be found 
at build time.

```
# Sample of setting $GOPATH
mkdir -p $HOME/go
export GOPATH="$HOME/go"

# Running 'go get -d' will clone the forklift-controller repo into the proper
# location on your $GOPATH
go get -d github.com/kubev2v/forklift

# Take a peek at the newly cloned files
ls -Fal $GOPATH/src/github.com/kubev2v/forklift
```

__5. Login __

```
$ oc login
```

__4. Create required CRDs __

TBD

---

__5.  Use `make run` to start the controller.__

```
$ make run

go generate ./pkg/... ./cmd/...
go fmt ./pkg/... ./cmd/...
go vet ./pkg/... ./cmd/...
go run ./cmd/forklift-controller/main.go
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up client for manager"}
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up manager"}
{"level":"info","ts":1555619493,"logger":"entrypoint","msg":"Registering Components."}

[...]
```

## Useful `make` targets

There are several useful Makefile targets for forklift-controller that developers
should be aware of.

| Command | Description |
| --- | --- |
| `run` | Build a controller manager binary and run the controller against the active cluster |
| `install` | Install generated CRDs onto the active cluster |
| `manifests` | Generate updated CRDs from types.go files, RBAC from annotations in controller, deploy manifest YAML |
| `crd-api-changelog` | Print CRD API changelog (Markdown). Optional `FROM_REF` (default: latest `v*` tag by version sort); optional `TO_REF` (default `HEAD`). Optional `SHOW_CHANGE_DIFFS=1` adds code diffs under **Changed** (can be large) |

### CRD schema changelog (optional)

[`hack/crd_changelog_diff.py`](hack/crd_changelog_diff.py) compares generated CRD OpenAPI under `operator/config/crd/bases/` between two git refs and prints changlog summaries as Markdown.

```bash
# Compare latest release tag → HEAD (defaults)
make crd-api-changelog

# Explicit range
make crd-api-changelog FROM_REF=v2.11.1 TO_REF=v2.11.2
# Optional: include unified diffs under Changed (larger output)
make crd-api-changelog FROM_REF=v2.11.1 TO_REF=v2.11.2 SHOW_CHANGE_DIFFS=1
```
