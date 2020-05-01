# Hacking on virt-controller

## Building and running virt-controller with `make run`

__1. Install prerequisites__

 - golang compiler (tested @ 1.11.5)
 - dep (tested @ v0.5.0)

__2. Clone the project to your `$GOPATH`__

Clone virt-controller to your $GOPATH so that dependencies in `vendor` will be found 
at build time.

```
# Sample of setting $GOPATH
mkdir -p $HOME/go
export GOPATH="$HOME/go"

# Running 'go get -d' will clone the virt-controller repo into the proper
# location on your $GOPATH
go get -d github.com/konveyor/virt-controller

# Take a peek at the newly cloned files
ls -Fal $GOPATH/src/github.com/konveyor/virt-controller
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
go run ./cmd/manager/main.go
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up client for manager"}
{"level":"info","ts":1555619492,"logger":"entrypoint","msg":"setting up manager"}
{"level":"info","ts":1555619493,"logger":"entrypoint","msg":"Registering Components."}

[...]
```

## Useful `make` targets

There are several useful Makefile targets for virt-controller that developers
should be aware of.

| Command | Description |
| --- | --- |
| `run` | Build a controller manager binary and run the controller against the active cluster |
| `manager` | Build a controller manager binary |
| `install` | Install generated CRDs onto the active cluster |
| `manifests` | Generate updated CRDs from types.go files, RBAC from annotations in controller, deploy manifest YAML |
| `docker-build` | Build the controller into a container image. Requires support for multi-stage builds, which may require moby-engine |
