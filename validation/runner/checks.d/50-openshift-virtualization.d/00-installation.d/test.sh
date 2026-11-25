#!/usr/bin/bash
#
# Copyright (C) 2025-2026 Red Hat, Inc.
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

oc get namespace openshift-cnv \
|| fail_with Operator "The openshift-cnv namespace does not exist. Did you install OpenShift Virtualization?"

oc get -n openshift-cnv kubevirt kubevirt-kubevirt-hyperconverged \
|| fail_with OperatorDeployment "The kubevirt config kubevirt-kubevirt-hyperconverged does not exist. Did you deploy the operator?"
