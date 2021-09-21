# Builder image
FROM registry.access.redhat.com/ubi8/go-toolset:1.15.13 as builder
ENV GOPATH=$APP_ROOT
COPY . .
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o manager github.com/konveyor/forklift-controller/cmd/manager


# Runner image
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.4
RUN microdnf -y install tar && microdnf clean all

COPY --from=builder /opt/app-root/src/manager /usr/local/bin/manager
ENV KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1
ENTRYPOINT ["/usr/local/bin/manager"]

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
