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

[ -n "${VIRT_VALIDATE_PROFILE:-}" ] \
  || exec skip_with "Canary workflow is only selected by a validation profile"

NS="${VIRT_VALIDATE_NAMESPACE:-}"
DATA_SOURCE="${VIRT_VALIDATE_DATA_SOURCE:-}"
VM_OPERATION_TIMEOUT="${VM_READY_TIMEOUT:-2m}"
CLEANUP_POLICY="${VIRT_VALIDATE_CLEANUP_POLICY:-Always}"
MANIFEST="$(mktemp)"
VMNAME=""
DVNAMES=""
SNAPSHOT_NAME=""
MIGRATION_NAME=""

record_step() {
  [ -n "${VIRT_VALIDATE_STEPS_FILE:-}" ] || return 0
  jq -cn --arg name "$1" --arg status "$2" --arg state "${3:-}" --arg message "${4:-}" \
    '{name: $name, status: $status} +
     (if $state == "" and $message == "" then {} else {extra: ({state: $state, message: $message} | with_entries(select(.value != "")))} end)' \
    >> "$VIRT_VALIDATE_STEPS_FILE"
}

fail_step() {
  record_step "$1" failed "" "$3"
  fail_with "$2" "$3"
}

retain_artifacts() {
  [ "$CLEANUP_POLICY" = "Never" ] || { [ "$CLEANUP_POLICY" = "OnFailure" ] && [ "$1" -ne 0 ] && [ "$1" -ne 77 ]; }
}

cleanup() {
  status=$?
  if retain_artifacts "$status"; then
    [ -n "${VIRT_VALIDATE_ARTIFACTS_FILE:-}" ] && {
      printf 'retainedNamespace=%s\n' "$NS"
      [ -n "$VMNAME" ] && printf 'retainedVM=%s\n' "$VMNAME"
      [ -n "$SNAPSHOT_NAME" ] && printf 'retainedSnapshot=%s\n' "$SNAPSHOT_NAME"
      [ -n "$MIGRATION_NAME" ] && printf 'retainedMigration=%s\n' "$MIGRATION_NAME"
      [ -n "$DVNAMES" ] && printf 'retainedDataVolumes=%s\nretainedPVCs=%s\n' "${DVNAMES% }" "${DVNAMES% }"
    } >> "$VIRT_VALIDATE_ARTIFACTS_FILE"
  else
    [ -n "$MIGRATION_NAME" ] && oc delete ${NS:+-n "$NS"} virtualmachineinstancemigration "$MIGRATION_NAME" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    [ -n "$SNAPSHOT_NAME" ] && oc delete ${NS:+-n "$NS"} virtualmachinesnapshot "$SNAPSHOT_NAME" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
    [ -n "$VMNAME" ] && oc delete ${NS:+-n "$NS"} vm,vmi "$VMNAME" --ignore-not-found=true --force --grace-period=0 --wait=true --timeout=60s >/dev/null 2>&1 || true
    for dv in $DVNAMES; do
      oc delete ${NS:+-n "$NS"} datavolume,pvc "$dv" --ignore-not-found=true --force --grace-period=0 --wait=false >/dev/null 2>&1 || true
    done
  fi
  rm -f "$MANIFEST"
  return "$status"
}
trap cleanup EXIT

record_step "Start VM" pending queued
record_step "Snapshot" pending queued
if [ "${VIRT_VALIDATE_PROFILE:-}" = "basic-v1" ] || [ "${VIRT_VALIDATE_PROFILE:-}" = "basic-all-nodes-v1" ]; then
  record_step "Live migrate VM" pending queued
fi
record_step "Start VM" pending running
if [ -z "$DATA_SOURCE" ]; then
  for source in rhel10 rhel9; do
    if oc get datasource -n openshift-virtualization-os-images "$source" >/dev/null 2>&1; then
      DATA_SOURCE="openshift-virtualization-os-images/$source"
      break
    fi
  done
fi
[ -n "$DATA_SOURCE" ] || fail_step "Start VM" Setup "No DataSource was configured or discovered"

virtctl create vm "--volume-import=type:ds,src:${DATA_SOURCE}" > "$MANIFEST" \
  || fail_step "Start VM" Setup "Failed to generate the canary VM manifest"
oc create --dry-run=client -o json -f "$MANIFEST" | \
  jq --arg namespace "$NS" --arg hostname "${VIRT_VALIDATE_NODE_HOSTNAME:-}" --arg validation_uid "${VIRT_VALIDATE_VALIDATION_UID:-}" '
    if $namespace != "" then .metadata.namespace = $namespace else . end |
    if $validation_uid != "" then .metadata.labels["forklift.konveyor.io/validation-uid"] = $validation_uid else . end |
    if $hostname != "" then .spec.template.spec.nodeSelector["kubernetes.io/hostname"] = $hostname else . end
  ' > "${MANIFEST}.json" \
  || fail_step "Start VM" Setup "Failed to prepare the canary VM manifest"
mv "${MANIFEST}.json" "$MANIFEST"
oc create ${NS:+-n "$NS"} -f "$MANIFEST" \
  || fail_step "Start VM" Setup "Failed to create the canary VM"

VMNAME=$(oc get ${NS:+-n "$NS"} -o jsonpath='{.metadata.name}' -f "$MANIFEST")
DVNAMES=$(oc get ${NS:+-n "$NS"} vm "$VMNAME" -o jsonpath='{range .spec.dataVolumeTemplates[*]}{.metadata.name}{" "}{end}')
oc wait ${NS:+-n "$NS"} --for=condition=Ready=true --timeout="$VM_OPERATION_TIMEOUT" -f "$MANIFEST" \
  || fail_step "Start VM" Scheduling "VM did not become Ready"

if [ -n "${VIRT_VALIDATE_NODE:-}" ]; then
  ACTUAL_NODE=$(oc get ${NS:+-n "$NS"} vmi "$VMNAME" -o jsonpath='{.status.nodeName}')
  [ "$ACTUAL_NODE" = "$VIRT_VALIDATE_NODE" ] \
    || fail_step "Start VM" Scheduling "VM landed on ${ACTUAL_NODE:-no node}, expected ${VIRT_VALIDATE_NODE}"
  [ -n "${VIRT_VALIDATE_ARTIFACTS_FILE:-}" ] && printf 'observedNodeName=%s\n' "$ACTUAL_NODE" >> "$VIRT_VALIDATE_ARTIFACTS_FILE"
fi
record_step "Start VM" passed

record_step "Snapshot" pending running
VSC_DELETE_COUNT=$(oc get volumesnapshotclass -o json 2>/dev/null | jq '[.items[] | select(.deletionPolicy == "Delete")] | length' 2>/dev/null || echo 0)
[ "$VSC_DELETE_COUNT" -gt 0 ] \
  || fail_step "Snapshot" VolumeSnapshotClass "No VolumeSnapshotClass with deletionPolicy Delete was found"
SNAPSHOT_NAME="snap-${VMNAME}"
cat > snapshot.yaml <<EOF
apiVersion: snapshot.kubevirt.io/v1alpha1
kind: VirtualMachineSnapshot
metadata:
  name: ${SNAPSHOT_NAME}
  labels:
    forklift.konveyor.io/validation-uid: "${VIRT_VALIDATE_VALIDATION_UID:-}"
spec:
  source:
    apiGroup: kubevirt.io
    kind: VirtualMachine
    name: ${VMNAME}
EOF
oc apply ${NS:+-n "$NS"} -f snapshot.yaml \
  || fail_step "Snapshot" Snapshot "Failed to create snapshot"
oc wait ${NS:+-n "$NS"} --for=condition=Ready --timeout="$VM_OPERATION_TIMEOUT" -f snapshot.yaml \
  || fail_step "Snapshot" Snapshot "Snapshot did not become Ready"
record_step "Snapshot" passed

if [ "${VIRT_VALIDATE_PROFILE:-}" = "basic-v1" ] || [ "${VIRT_VALIDATE_PROFILE:-}" = "basic-all-nodes-v1" ]; then
  record_step "Live migrate VM" pending running
  oc auth can-i create virtualmachineinstancemigrations.kubevirt.io ${NS:+-n "$NS"} | grep -qx yes \
    || fail_step "Live migrate VM" Permissions "No permission to perform live migration"
  if [ "${VIRT_VALIDATE_PROFILE:-}" = "basic-all-nodes-v1" ]; then
    oc patch ${NS:+-n "$NS"} vm "$VMNAME" --type=json -p '[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]' \
      || fail_step "Live migrate VM" Scheduling "Failed to remove the canary VM node selector"
    export NS VMNAME
    timeout "$VM_OPERATION_TIMEOUT" bash -c '
      while ! {
        if [ -n "$NS" ]; then
          oc -n "$NS" get vmi "$VMNAME" -o json
        else
          oc get vmi "$VMNAME" -o json
        fi | jq -e '\''.spec.nodeSelector["kubernetes.io/hostname"] == null'\'' >/dev/null
      }; do sleep 1; done
    ' || fail_step "Live migrate VM" Scheduling "Canary node selector did not propagate to the VMI; check the RestartRequired condition"
  fi
  SOURCE_NODE=$(oc get ${NS:+-n "$NS"} vmi "$VMNAME" -o jsonpath='{.status.nodeName}')
  [ -n "$SOURCE_NODE" ] || fail_step "Live migrate VM" Scheduling "VM has no source node"
  MIGRATION_NAME="${VMNAME}-migration"
  cat > migration.yaml <<EOF
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  name: ${MIGRATION_NAME}
  labels:
    forklift.konveyor.io/validation-uid: "${VIRT_VALIDATE_VALIDATION_UID:-}"
spec:
  vmiName: ${VMNAME}
EOF
  oc apply ${NS:+-n "$NS"} -f migration.yaml \
    || fail_step "Live migrate VM" Migration "Failed to create migration"
  oc wait ${NS:+-n "$NS"} --for=jsonpath='{.status.phase}'=Succeeded --timeout="$VM_OPERATION_TIMEOUT" -f migration.yaml \
    || fail_step "Live migrate VM" Migration "Migration did not succeed"
  TARGET_NODE=$(oc get ${NS:+-n "$NS"} vmi "$VMNAME" -o jsonpath='{.status.nodeName}')
  [ -n "$TARGET_NODE" ] && [ "$TARGET_NODE" != "$SOURCE_NODE" ] \
    || fail_step "Live migrate VM" Migration "VM remained on ${SOURCE_NODE:-no node} after migration"
  [ -n "${VIRT_VALIDATE_ARTIFACTS_FILE:-}" ] && {
    printf 'migrationSourceNode=%s\n' "$SOURCE_NODE"
    printf 'migrationTargetNode=%s\n' "$TARGET_NODE"
  } >> "$VIRT_VALIDATE_ARTIFACTS_FILE"
  record_step "Live migrate VM" passed
fi
