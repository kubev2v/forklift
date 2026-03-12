.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

# Build flags
PROD_LDFLAGS := -buildvcs=false -ldflags="-w -s"
DEV_LDFLAGS := 

.PHONY: build
build: fmt vet
	go build $(DEV_LDFLAGS) -o bin/kubectl-mtv

.PHONY: build-linux
build-linux: fmt vet
	GOOS=linux GOARCH=amd64 go build $(PROD_LDFLAGS) -o bin/kubectl-mtv-linux-amd64
	GOOS=linux GOARCH=arm64 go build $(PROD_LDFLAGS) -o bin/kubectl-mtv-linux-arm64

.PHONY: build-windows
build-windows: fmt vet
	GOOS=windows GOARCH=amd64 go build $(PROD_LDFLAGS) -o bin/kubectl-mtv-windows-amd64.exe

.PHONY: build-darwin
build-darwin: fmt vet
	GOOS=darwin GOARCH=amd64 go build $(PROD_LDFLAGS) -o bin/kubectl-mtv-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build $(PROD_LDFLAGS) -o bin/kubectl-mtv-darwin-arm64

.PHONY: build-all
build-all: build-linux build-windows build-darwin
	@echo "All platform binaries built successfully"

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: update-kubectl-mtv
update-kubectl-mtv:
	./update-kubectl-mtv.sh

.PHONY: clean
clean:
	rm -rf bin/ vendor/

