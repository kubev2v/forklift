#!/bin/bash
# This script bypasses OLM upgrade path and updates MTV to a specific version
# This should only serve as a reference, cluster admin should know what they are doing.

# Prerequisites:
#   installed "oc" and "jq" cli tools
#   logged into the cluster

# Defaults
MTV_SUBSCRIPTION=mtv-operator
MTV_NAMESPACE=openshift-mtv
MTV_VERSION=2.9.2 # change to the version you want to upgrade to
CATALOG_SOURCE=redhat-operators

# Get installed CSV from current subscription
CSV=$(oc get subscription $MTV_SUBSCRIPTION -n $MTV_NAMESPACE -o json | jq -r '.status.installedCSV')
# Remove the current subscription
oc delete subscription -n $MTV_NAMESPACE $MTV_SUBSCRIPTION
# Remove the current CSV
oc delete csv -n $MTV_NAMESPACE $CSV
# Remove the current Operator CR
oc delete operators -n $MTV_NAMESPACE "$MTV_SUBSCRIPTION.$MTV_NAMESPACE"

# Create a new subscription with patched version
echo """apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  labels:
    operators.coreos.com/$MTV_SUBSCRIPTION.$MTV_NAMESPACE: \"\"
  name: $MTV_SUBSCRIPTION
  namespace: $MTV_NAMESPACE
spec:
  channel: release-v${MTV_VERSION%.*}
  installPlanApproval: Manual
  name: $MTV_SUBSCRIPTION
  source: $CATALOG_SOURCE
  sourceNamespace: openshift-marketplace
  startingCSV: mtv-operator.v$MTV_VERSION
""" > new_subscription.yaml

# Apply the new subscription
oc apply -n $MTV_NAMESPACE -f new_subscription.yaml
# Leave the file on the host for utility

### Further section is optional and can be removed to disable automatic Install Plan approval 
# Wait for OLM to create an Install Plan
sleep 2
ip=$(oc get -n $MTV_NAMESPACE subs -o json $MTV_SUBSCRIPTION | jq '.status.installplan.name' -r)
# Approve the install plan
oc -n $MTV_NAMESPACE patch installplan $ip -p '{"spec":{"approved":true}}' --type merge
