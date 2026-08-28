# Guest conversion on non-OpenShift clusters

Guest conversion runs `virt-v2v`, which launches a libguestfs appliance.
libguestfs gives that appliance a network through `passt`, and `passt`
sandboxes itself before it starts forwarding: it creates a user namespace,
detaches the mount, IPC and UTS namespaces, then remounts `/` and pivots into
an empty tmpfs.

The conversion pod runs as uid 107 with every capability dropped. Two of the
node's default confinement settings deny that sandbox:

| Barrier | Rule | Symptom |
|---|---|---|
| seccomp | The runtime default profile allows `unshare(2)` only for callers holding `CAP_SYS_ADMIN` | `Couldn't create user namespace: Operation not permitted` |
| AppArmor | The runtime default profile carries `deny mount,` | `Failed to remount /: Permission denied` |

They are independent, and clearing only the first moves the failure to the
second. OpenShift hits neither: its nodes carry a seccomp profile that permits
`unshare`, and they confine containers with SELinux rather than AppArmor.

Raising the pod's privileges does not help. A privileged conversion pod holds
`CapEff: 000001ffffffffff` and can `unshare` every namespace, yet `passt` still
fails, because it drops root *before* it sandboxes itself and no longer holds
`CAP_SYS_ADMIN` at the point it needs it. The constraint is the syscall filter
on the non-root path, not the capability set.

## Enabling conversion

The controller reads four settings, all unset by default. Leaving them unset
keeps today's behaviour on every platform, because naming a profile that a node
does not carry makes the kubelet refuse to start the pod at all.

| Setting | Values | Default |
|---|---|---|
| `VIRT_V2V_SECCOMP_PROFILE_TYPE` | `RuntimeDefault`, `Localhost`, `Unconfined` | unset: `Localhost` on OpenShift, `RuntimeDefault` elsewhere |
| `VIRT_V2V_SECCOMP_PROFILE_PATH` | path below the kubelet seccomp root | `profiles/unshare.json` |
| `VIRT_V2V_APPARMOR_PROFILE_TYPE` | `RuntimeDefault`, `Localhost`, `Unconfined` | unset: field is not set on the pod |
| `VIRT_V2V_APPARMOR_PROFILE_PATH` | name of a profile loaded on the node | `forklift-virt-v2v-unshare` |

On the `ForkliftController` CR the same settings are
`virt_v2v_seccomp_profile_type`, `virt_v2v_seccomp_profile_path`,
`virt_v2v_apparmor_profile_type` and `virt_v2v_apparmor_profile_path`.

### 1. Put the profiles on the nodes

Both `Localhost` types name something that must already exist on every node
that can run a conversion. Use the Security Profiles Operator if you already
run it. Otherwise `hack/seccomp/` carries the two profiles and a DaemonSet that
installs them:

```sh
kubectl -n konveyor-forklift create configmap forklift-virt-v2v-profiles \
  --from-file=hack/seccomp/unshare.json \
  --from-file=hack/seccomp/apparmor-unshare.profile
kubectl apply -f hack/seccomp/profile-installer.yaml
kubectl -n konveyor-forklift rollout status ds/forklift-virt-v2v-profile-installer
```

`hack/seccomp/unshare.json` is the container runtime's default seccomp profile
with `unshare`, `mount`, `umount2`, `pivot_root`, `chroot` and `setns` allowed
unconditionally rather than only under `CAP_SYS_ADMIN`.
`hack/seccomp/apparmor-unshare.profile` is the runtime's default AppArmor
profile with the mounts that `passt` performs permitted.

### 2. Point the controller at them

Set the fields on the `ForkliftController` CR. Mistakes are rejected as the
CR is saved, and the operator carries the values onto the controller:

```sh
kubectl -n konveyor-forklift patch forkliftcontroller forklift-controller \
  --type merge -p '{"spec":{
    "virt_v2v_seccomp_profile_type": "Localhost",
    "virt_v2v_seccomp_profile_path": "profiles/unshare.json",
    "virt_v2v_apparmor_profile_type": "Localhost",
    "virt_v2v_apparmor_profile_path": "forklift-virt-v2v-unshare"}}'
```

Nodes that do not enforce AppArmor need only the seccomp pair; leave the
AppArmor settings unset there.

On a deployment without the operator, set the `VIRT_V2V_*` environment
variables from the table above on the controller Deployment instead. That
path has no admission check: a path without its type stops the controller
at startup, with the reason in the controller log.

## Checking a node before you migrate

Whether a node needs the AppArmor half is worth establishing directly, since it
depends on the distribution rather than on Kubernetes:

```sh
kubectl run unshare-probe --rm -it --restart=Never \
  --image=registry.k8s.io/busybox:1.27.2 \
  --overrides='{"spec":{"securityContext":{"runAsUser":107,"runAsNonRoot":true,
    "seccompProfile":{"type":"Localhost","localhostProfile":"profiles/unshare.json"}},
    "containers":[{"name":"probe","image":"registry.k8s.io/busybox:1.27.2",
    "command":["sh","-c","unshare -Urm true && echo SANDBOX_OK"],
    "securityContext":{"allowPrivilegeEscalation":false,
    "capabilities":{"drop":["ALL"]}}}]}}'
```

`SANDBOX_OK` means the seccomp profile alone is enough. A failure on the `-m`
(mount namespace) step means AppArmor is also mediating and the node needs the
AppArmor profile too.

## Related issues

- https://github.com/kubev2v/forklift/issues/4997 — vanilla Kubernetes 1.33, vSphere source
- https://github.com/kubev2v/forklift/issues/4491 — Harvester
- https://github.com/kubev2v/forklift/issues/1942 — the pod-start failure that led to gating the profile on OpenShift

The Kyverno policy suggested on #4491, which makes the conversion pod
privileged, does not work: it was tested against both `:latest` and
`release-2.8` and fails identically, for the reason given above.
