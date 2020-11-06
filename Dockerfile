# Builder image
FROM registry.access.redhat.com/ubi8/go-toolset:1.14.7 as builder
ENV GOPATH=$APP_ROOT
COPY pkg    $APP_ROOT/src/github.com/konveyor/virt-controller/pkg
COPY cmd    $APP_ROOT/src/github.com/konveyor/virt-controller/cmd
COPY vendor $APP_ROOT/src/github.com/konveyor/virt-controller/vendor
RUN CGO_ENABLED=1 GOOS=linux go build -a -o manager github.com/konveyor/virt-controller/cmd/manager


# Runner image
FROM registry.access.redhat.com/ubi8-minimal

LABEL name="konveyor/virt-controller" \
      description="Konveyor for Virtualization - Controller" \
      help="For more information visit https://konveyor.io" \
      license="Apache License 2.0" \
      maintainer="jortel@redhat.com" \
      summary="Konveyor for Virtualization - Controller" \
      url="https://quay.io/repository/konveyor/virt-controller" \
      usage="podman run konveyor/virt-controller:latest" \
      com.redhat.component="konveyor-virt-controller-container" \
      io.k8s.display-name="virt-controller" \
      io.k8s.description="Konveyor for Virtualization - Controller" \
      io.openshift.expose-services="" \
      io.openshift.tags="operator,konveyor,controller" \
      io.openshift.min-cpu="100m" \
      io.openshift.min-memory="350Mi"

COPY --from=builder /opt/app-root/src/manager /usr/local/bin/manager

ENTRYPOINT ["/usr/local/bin/manager"]
