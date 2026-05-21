MTV_VERSION ?= v$(shell grep '^MTV_VERSION=' ../../build/release.conf 2>/dev/null | cut -d= -f2 || echo "0.0.0")
LICENSE_FILE := ../../LICENSE

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

.PHONY: dist-linux-amd64
dist-linux-amd64:
	@mkdir -p dist
	@cp $(LICENSE_FILE) bin/LICENSE
	tar -zcf dist/kubectl-mtv-$(MTV_VERSION)-linux-amd64.tar.gz -C bin kubectl-mtv-linux-amd64 LICENSE
	@rm -f bin/LICENSE

.PHONY: dist-linux-arm64
dist-linux-arm64:
	@mkdir -p dist
	@cp $(LICENSE_FILE) bin/LICENSE
	tar -zcf dist/kubectl-mtv-$(MTV_VERSION)-linux-arm64.tar.gz -C bin kubectl-mtv-linux-arm64 LICENSE
	@rm -f bin/LICENSE

.PHONY: dist-darwin-amd64
dist-darwin-amd64:
	@mkdir -p dist
	@cp $(LICENSE_FILE) bin/LICENSE
	tar -zcf dist/kubectl-mtv-$(MTV_VERSION)-darwin-amd64.tar.gz -C bin kubectl-mtv-darwin-amd64 LICENSE
	@rm -f bin/LICENSE

.PHONY: dist-darwin-arm64
dist-darwin-arm64:
	@mkdir -p dist
	@cp $(LICENSE_FILE) bin/LICENSE
	tar -zcf dist/kubectl-mtv-$(MTV_VERSION)-darwin-arm64.tar.gz -C bin kubectl-mtv-darwin-arm64 LICENSE
	@rm -f bin/LICENSE

.PHONY: dist-windows-amd64
dist-windows-amd64:
	@mkdir -p dist
	@cp $(LICENSE_FILE) bin/LICENSE
	@cd bin && zip -q ../dist/kubectl-mtv-$(MTV_VERSION)-windows-amd64.zip kubectl-mtv-windows-amd64.exe LICENSE
	@rm -f bin/LICENSE

.PHONY: dist-checksums
dist-checksums:
	@cd dist && for f in *.tar.gz *.zip; do sha256sum "$$f" > "$$f.sha256sum"; done

.PHONY: dist-all
dist-all: build-all dist-linux-amd64 dist-linux-arm64 dist-darwin-amd64 dist-darwin-arm64 dist-windows-amd64 dist-checksums
	@echo "Release archives created in dist/:"
	@ls -1 dist/

.PHONY: dist-clean
dist-clean:
	rm -rf dist/

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: update-kubectl-mtv
update-kubectl-mtv:
	./update-kubectl-mtv.sh

.PHONY: clean
clean: dist-clean
	rm -rf bin/ vendor/

