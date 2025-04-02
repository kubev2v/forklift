# as of January 2025 go 1.22 is still the latest go-toolset for ubi-9
# but since trident depends on 1.23 we must install it.
# go-toolset 1.23 on ubi-9 is targeted to ubi 9.6 which is around May 2025
#FROM registry.access.redhat.com/ubi8/go-toolset:1.23.1 AS builder

From docker.io/golang:1.23 AS builder
USER root
WORKDIR /usr/src/app
COPY . .
RUN make build


