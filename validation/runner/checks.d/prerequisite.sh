#!/usr/bin/bash
#
# Copyright (C) 2026 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Global prerequisite check
# This runs before ANY tests execute. If it fails, no tests will run.
#
# Common use cases:
# - Verify cluster connectivity (oc whoami)
# - Check that required operators are installed
# - Validate minimum cluster version
# - Ensure required permissions are available

set -x

# Verify we're logged into an OpenShift cluster
WHOAMI=$(oc whoami 2>&1) || fail_with "Cannot authenticate to the target OpenShift API: ${WHOAMI}"

# Report success
pass_with info "Cluster connection verified as $(oc whoami)"
