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

.PHONY: actions action-help
actions: ## Run all GitHub Action checks that run on a pull request creation
	@echo "Running all GitHub Action checks for pull request events..."
	@act -l | grep -v ^Stage | grep pull_request | grep -v image_security_scan | awk '{print $$2}' | while read WF; do \
		echo "Running workflow: $${WF}"; \
		act pull_request --no-cache-server --platform ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest --job "$${WF}"; \
	done

action-help: ## Echo instructions to run one specific workflow locally
	@echo "GitHub Workflows can be run locally with the following command:"
	@echo "act pull_request --no-cache-server --platform ubuntu-latest=ghcr.io/catthehacker/ubuntu:act-latest --job <jobid>"
	@echo ""
	@echo "Where '<jobid>' is a Job ID returned by the command:"
	@echo "act -l"
	@echo ""
	@echo "NOTE: if act is not installed, it can be downloaded from https://github.com/nektos/act"