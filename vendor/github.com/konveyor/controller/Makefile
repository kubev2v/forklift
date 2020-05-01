GOOS ?= `go env GOOS`

# Run tests
test: generate fmt vet
	go test ./pkg/... -coverprofile cover.out

# Run go fmt against code
fmt:
	go fmt ./pkg/...

# Run go vet against code
vet:
	go vet ./pkg/...

# Generate code
generate:
	go generate ./pkg/...
