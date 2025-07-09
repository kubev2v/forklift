# Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Args for unit test run
FORMAT=pretty
OUTFILE=
TEST_TAGS=
TEST_PATHS=unittest

# Port for dlv debugger
debug_port=55555

# These lists contain applicable files 
srcfiles=		authenticate.go interface.go replication.go system.go sloprovisioning.go volume_snapshot.go volume_replication.go metrics.go migration.go file.go
integrationfiles=	inttest/pmax_integration_test.go inttest/pmax_replication_integration_test.go
unitfiles=		unit_test.go unit_steps_test.go

# These variables should be set for your Unisphere installation
Endpoint="https://1.1.1.1:8443"
Username=			# Leave blank for the default username
Password=			# Leave blank for the default password
APIVersion=""                   # Leave blank for default APIVersion
DefaultStorageGroup="CSI-Integration-Test"
DefaultStoragePool="SRP_1"
VolumePrefix="XX"		# Use a two letter initial sequence to identify your files
SymmetrixID="000000000001"

all: unit-test int-test check

unit-test:
	go clean -cache
	APIVersion=$(APIVersion) \
	go test -v -coverprofile=c.out $(unitfiles) $(srcfiles) -args "format=$(FORMAT)" "outfile=${OUTFILE}" "test-tags=$(TEST_TAGS)" "test_paths=$(TEST_PATHS)"

unit-test-debug-build:
	go clean -cache
	go build -gcflags "all=-N -l" $(unitfiles) $(srcfiles)

dlv-unit-test:
	echo "Starting test with debugging port open (attache debugger. ctrl-C to exit out after debugging) ..."
	APIVersion=$(APIVersion) \
	dlv --listen=localhost:$(debug_port) --headless=true --api-version=2 --accept-multiclient exec pmax.test.exe

unit-test-debug: unit-test-debug-build dlv-unit-test

int-test:
	bash inttest/run_int.sh 

int-test-no-cleanup:
	bash inttest/run_int.sh --no-cleanup

short-int-test: 
	bash inttest/run_int.sh --short

gocover:
	go tool cover -html=c.out

check:
	gofmt -w $(srcfiles) $(unitfiles) $(integrationfiles)
	golint -set_exit_status
	go vet
