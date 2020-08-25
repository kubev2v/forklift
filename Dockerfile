# Build the manager binary
FROM golang:1.14.4 as builder

# Copy in the go src
WORKDIR /go/src/github.com/konveyor/virt-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -a -o manager github.com/konveyor/virt-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM registry.access.redhat.com/ubi8-minimal
WORKDIR /
COPY --from=builder /go/src/github.com/konveyor/virt-controller/manager .
ENTRYPOINT ["/manager"]
