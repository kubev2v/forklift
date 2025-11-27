# MTV Ansible Hook Examples

This directory contains simple, working examples of Ansible playbooks for Migration Toolkit for Virtualization (MTV) hooks.

## What are MTV Hooks?

MTV hooks let you run Ansible playbooks at specific points during VM migration:
- **PreHook**: Runs before migration starts (e.g., prepare the VM)
- **PostHook**: Runs after migration completes (e.g., install monitoring, cleanup)

## Quick Start

### 1. Set up SSH access to your VMs

Create a Kubernetes secret with your SSH private key:

```bash
kubectl create secret generic vm-ssh-credentials \
  --from-file=key=/path/to/your/private/key \
  -n konveyor-forklift
```

### 2. Choose an example

- **prehook-cloud-init/**: Install cloud-init before migration
- **posthook-monitoring/**: Install node_exporter monitoring after migration

### 3. Apply the Hook CR

```bash
kubectl apply -f prehook-cloud-init/hook-cr.yaml
```

### 4. Reference the hook in your migration Plan

```yaml
spec:
  vms:
    - id: vm-123
      hooks:
        - hook:
            namespace: konveyor-forklift
            name: install-cloud-init
          step: PreHook
```

## How It Works

1. MTV creates a Job in the cluster when the hook is triggered
2. The job runs the hook-runner container with your playbook
3. The playbook connects to your VM via SSH
4. Ansible tasks execute on the VM
5. Migration continues after hook completes

## Customizing for Your Environment

### Update VM connection details

Each playbook connects to the VM using information from MTV. The VM's IP address is available in the `workload.vm.ipaddress` variable.

### Modify SSH credentials

Update the secret name in playbooks:
```yaml
- k8s_info:
    api_version: v1
    kind: Secret
    name: vm-ssh-credentials  # Change this to your secret name
    namespace: konveyor-forklift
  register: ssh_creds
```

### Change the SSH user

Update the `ansible_user` in playbooks:
```yaml
- add_host:
    name: "{{ workload.vm.ipaddress }}"
    ansible_user: root  # Change to your SSH user
    groups: target_vms
```

## Creating Your Own Playbook

1. Write your Ansible playbook (see examples)
2. Encode it to base64:
   ```bash
   ./scripts/encode-playbook.sh your-playbook.yml
   ```
3. Create a Hook CR with the encoded playbook
4. Apply it and reference it in your Plan

## Troubleshooting

### Hook job fails to start
- Check that the hook-runner image is accessible: `quay.io/konveyor/hook-runner:latest`
- Verify ServiceAccount permissions (default: `forklift-controller`)

### Cannot connect to VM
- Verify SSH credentials are correct
- Check that the VM's IP is accessible from the cluster
- Ensure the SSH key has proper permissions (0600)
- Verify the SSH user exists on the VM

### Playbook tasks fail
- Check hook job logs: `kubectl logs -n konveyor-forklift job/<hook-job-name>`
- Verify the VM has required packages/dependencies
- Check that the user has sudo permissions if needed

## Requirements

- MTV 2.6 or later
- SSH access to source VMs
- Network connectivity from the cluster to VMs

## Contributing

Found an issue or have a useful example to share? Please open an issue or PR in the main forklift repository.

