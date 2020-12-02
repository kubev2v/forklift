# Builder image
FROM registry.access.redhat.com/ubi8/go-toolset:1.14.7 as builder
ENV GOPATH=$APP_ROOT
COPY pkg    $APP_ROOT/src/github.com/konveyor/forklift-controller/pkg
COPY cmd    $APP_ROOT/src/github.com/konveyor/forklift-controller/cmd
COPY vendor $APP_ROOT/src/github.com/konveyor/forklift-controller/vendor
RUN GO111MODULE=off CGO_ENABLED=1 GOOS=linux go build -a -o manager github.com/konveyor/forklift-controller/cmd/manager


# Runner image
FROM registry.access.redhat.com/ubi8-minimal

LABEL name="konveyor/forklift-controller" \
      description="Konveyor Forklift - Controller" \
      help="For more information visit https://konveyor.io" \
      license="Apache License 2.0" \
      maintainer="jortel@redhat.com" \
      summary="Konveyor Forklift - Controller" \
      url="https://quay.io/repository/konveyor/forklift-controller" \
      usage="podman run konveyor/forklift-controller:latest" \
      com.redhat.component="konveyor-forklift-controller-container" \
      io.k8s.display-name="forklift-controller" \
      io.k8s.description="Konveyor Forklift - Controller" \
      io.openshift.expose-services="" \
      io.openshift.tags="operator,konveyor,forklift,controller" \
      io.openshift.min-cpu="100m" \
      io.openshift.min-memory="350Mi"

COPY --from=builder /opt/app-root/src/manager /usr/local/bin/manager

ENTRYPOINT ["/usr/local/bin/manager"]
