integration_tests_path=./inttests
unit_test_paths= ./ ./api

all: unit-test int-test mock-test check gosec

unit-test:
	go clean -cache
	go test -v -coverprofile=c.out $(unit_test_paths)

int-test:
	@bash $(integration_tests_path)/run-integration.sh

gocover:
	go tool cover -html=c.out

check: gosec
	gofmt -w ./.
	golint ./...
	go vet

gosec:
	gosec -quiet -log gosec.log -out=gosecresults.csv -fmt=csv ./...
