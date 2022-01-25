# Introduction
Migration hooks provide a means for running custom code at points in the migration to handle logic unique to a migration. This may include writing new configuration files or installing additional packages in preparation for migration to prepare for the new environment.

# Default hook image
The default hook image is `quay.io/konveyor/hook-runner:latest`. It is based off of the Ansible Runner image with the addition of python-openshift to provided Ansible k8s resources as well as a recent oc binary.

# Hook execution
When an Ansible playbook is provided as part of a migration hook it will be mounted into the hook container as a ConfigMap. In either case the hook container will be run as job on the desired cluster, using the default ServiceAccount in the konveyor-forklift namespace.

# Adding a hook to a Plan
Hooks can be specified per VM and may be run as a post of pre hook. When adding a hook you must specify the namespace where the hook CR is located along with its name and specify whether it should be run as a PreHook or PostHook.

```
kind: Plan
apiVersion: forklift.konveyor.io/v1beta1
metadata:
  name: test
  namespace: konveyor-forklift
spec:
  vms:
    - id: vm-2861
      hooks:
        - hook:
            namespace: konveyor-forklift
            name: playbook
          step: PreHook
...
```

# Adding a Hook CR
The Hook CR represents a hook and an example is provided below. The playbook is base64 encoded.

You may also specify a serviceAccount to run the hook with in order to control access to resources on the cluster as desired.

To encode a playbook cat a file and pipe it for base64, for example `cat playbook.yml | base64 -w0`.

It is also possible to use a here doc:
```
cat << EOF | base64 -w0
- hosts: localhost
  tasks:
  - debug:
      msg: test
EOF
```

To decode an attached playbook retrieve the resource with custom output and pipe it to base64, for example `oc get -n konveyor-forklift hook playbook -o go-template='{{ .spec.playbook }}' | base64 -d`.

Hook Example:
```
apiVersion: forklift.konveyor.io/v1beta1
kind: Hook
metadata:
  name: playbook
  namespace: konveyor-forklift
spec:
  image: quay.io/konveyor/hook-runner
  playbook: LSBuYW1lOiBNYWluCiAgaG9zdHM6IGxvY2FsaG9zdAogIHRhc2tzOgogIC0gbmFtZTogTG9hZCBQbGFuCiAgICBpbmNsdWRlX3ZhcnM6CiAgICAgIGZpbGU6IHBsYW4ueW1sCiAgICAgIG5hbWU6IHBsYW4KCiAgLSBuYW1lOiBMb2FkIFdvcmtsb2FkCiAgICBpbmNsdWRlX3ZhcnM6CiAgICAgIGZpbGU6IHdvcmtsb2FkLnltbAogICAgICBuYW1lOiB3b3JrbG9hZAoKICAtIG5hbWU6IAogICAgZ2V0ZW50OgogICAgICBkYXRhYmFzZTogcGFzc3dkCiAgICAgIGtleTogInt7IGFuc2libGVfdXNlcl9pZCB9fSIKICAgICAgc3BsaXQ6ICc6JwoKICAtIG5hbWU6IEVuc3VyZSBTU0ggZGlyZWN0b3J5IGV4aXN0cwogICAgZmlsZToKICAgICAgcGF0aDogfi8uc3NoCiAgICAgIHN0YXRlOiBkaXJlY3RvcnkKICAgICAgbW9kZTogMDc1MAogICAgZW52aXJvbm1lbnQ6CiAgICAgIEhPTUU6ICJ7eyBhbnNpYmxlX2ZhY3RzLmdldGVudF9wYXNzd2RbYW5zaWJsZV91c2VyX2lkXVs0XSB9fSIKCiAgLSBrOHNfaW5mbzoKICAgICAgYXBpX3ZlcnNpb246IHYxCiAgICAgIGtpbmQ6IFNlY3JldAogICAgICBuYW1lOiBzc2gtY3JlZGVudGlhbHMKICAgICAgbmFtZXNwYWNlOiBrb252ZXlvci1mb3JrbGlmdAogICAgcmVnaXN0ZXI6IHNzaF9jcmVkZW50aWFscwoKICAtIG5hbWU6IENyZWF0ZSBTU0gga2V5CiAgICBjb3B5OgogICAgICBkZXN0OiB+Ly5zc2gvaWRfcnNhCiAgICAgIGNvbnRlbnQ6ICJ7eyBzc2hfY3JlZGVudGlhbHMucmVzb3VyY2VzWzBdLmRhdGEua2V5IHwgYjY0ZGVjb2RlIH19IgogICAgICBtb2RlOiAwNjAwCgogIC0gYWRkX2hvc3Q6CiAgICAgIG5hbWU6ICJ7eyB3b3JrbG9hZC52bS5pcGFkZHJlc3MgfX0iCiAgICAgIGFuc2libGVfdXNlcjogcm9vdAogICAgICBncm91cHM6IHZtcwoKLSBob3N0czogdm1zCiAgdGFza3M6CiAgLSBuYW1lOiBJbnN0YWxsIGNsb3VkLWluaXQKICAgIGRuZjoKICAgICAgbmFtZToKICAgICAgLSBjbG91ZC1pbml0CiAgICAgIHN0YXRlOiBsYXRlc3QKCiAgLSBuYW1lOiBDcmVhdGUgVGVzdCBGaWxlCiAgICBjb3B5OgogICAgICBkZXN0OiAvdGVzdC50eHQKICAgICAgY29udGVudDogIkhlbGxvIFdvcmxkIgogICAgICBtb2RlOiAwNjQ0Cg==
  serviceAccount: forklift-controller
```

# Storing additional information in secrets and configMaps
If you wish to access additional information stored in secrets or configMaps it is possible to retrieve it using k8s modules.

# Examples

## Install cloud-init on a VM and write a file before migration

### Create a secret with an SSH private key for the VM
Either use an existing key or generate a key pair, install the public key on the VM and base64 encode the private key in the secret.

```
apiVersion: v1
data:
  key: VGhpcyB3YXMgZ2VuZXJhdGVkIHdpdGggc3NoLWtleWdlbiBwdXJlbHkgZm9yIHRoaXMgZXhhbXBsZS4KSXQgaXMgbm90IHVzZWQgYW55d2hlcmUuCi0tLS0tQkVHSU4gT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0tCmIzQmxibk56YUMxclpYa3RkakVBQUFBQUJHNXZibVVBQUFBRWJtOXVaUUFBQUFBQUFBQUJBQUFCbHdBQUFBZHpjMmd0Y24KTmhBQUFBQXdFQUFRQUFBWUVBMzVTTFRReDBFVjdPTWJQR0FqcEsxK2JhQURTTVFuK1NBU2pyTGZLNWM5NGpHdzhDbnA4LwovRHErZHFBR1pxQkg2ZnAxYmVJM1BZZzVWVDk0RVdWQ2RrTjgwY3dEcEo0Z1R0NHFUQ1gzZUYvY2x5VXQyUC9zaTNjcnQ0CjBQdi9wVnZXU1U2TlhHaDJIZC93V0MwcGh5Z0RQOVc5SHRQSUF0OFpnZmV2ZnUwZHpraVl6OHNVaElWU2ZsRGpaNUFqcUcKUjV2TVVUaGlrczEvZVlCeTdiMkFFSEdzYU8xN3NFbWNiYUlHUHZuUFVwWmQrdjkyYU1JdWZoYjhLZkFSbzZ3Ty9ISW1VbQovdDdHWFBJUmxBMUhSV0p1U05odTQzZS9DY3ZYd3Z6RnZrdE9kYXlEQzBMTklHMkpVaURlNWd0UUQ1WHZXc1p3MHQvbEs1CklacjFrZXZRNUJsYWNISmViV1ZNYUQvdllpdFdhSFo4OEF1Y0czaGh2bjkrOGNSTGhNVExiVlFSMWh2UVpBL1JtQXN3eE0KT3VJSmRaUmtxTThLZlF4Z28zQThRNGJhQW1VbnpvM3Zwa0FWdC9uaGtIOTRaRE5rV2U2RlRhdThONStyYTJCZkdjZVA4VApvbjFEeTBLRlpaUlpCREVVRVc0eHdTYUVOYXQ3c2RDNnhpL1d5OURaQUFBRm1NRFBXeDdBejFzZUFBQUFCM056YUMxeWMyCkVBQUFHQkFOK1VpMDBNZEJGZXpqR3p4Z0k2U3RmbTJnQTBqRUova2dFbzZ5M3l1WFBlSXhzUEFwNmZQL3c2dm5hZ0JtYWcKUituNmRXM2lOejJJT1ZVL2VCRmxRblpEZk5ITUE2U2VJRTdlS2t3bDkzaGYzSmNsTGRqLzdJdDNLN2VORDcvNlZiMWtsTwpqVnhvZGgzZjhGZ3RLWWNvQXovVnZSN1R5QUxmR1lIM3IzN3RIYzVJbU0vTEZJU0ZVbjVRNDJlUUk2aGtlYnpGRTRZcExOCmYzbUFjdTI5Z0JCeHJHanRlN0JKbkcyaUJqNzV6MUtXWGZyL2RtakNMbjRXL0Nud0VhT3NEdnh5SmxKdjdleGx6eUVaUU4KUjBWaWJrallidU4zdnduTDE4TDh4YjVMVG5Xc2d3dEN6U0J0aVZJZzN1WUxVQStWNzFyR2NOTGY1U3VTR2E5WkhyME9RWgpXbkJ5WG0xbFRHZy83MklyVm1oMmZQQUxuQnQ0WWI1L2Z2SEVTNFRFeTIxVUVkWWIwR1FQMFpnTE1NVERyaUNYV1VaS2pQCkNuME1ZS053UEVPRzJnSmxKODZONzZaQUZiZjU0WkIvZUdRelpGbnVoVTJydkRlZnEydGdYeG5Iai9FNko5UTh0Q2hXV1UKV1FReEZCRnVNY0VtaERXcmU3SFF1c1l2MXN2UTJRQUFBQU1CQUFFQUFBR0JBSlZtZklNNjdDQmpXcU9KdnFua2EvakRrUwo4TDdpSE5mekg1TnRZWVdPWmRMTlk2L0lRa1pDeFcwTWtSKzlUK0M3QUZKZzBNV2Q5ck5PeUxJZDkxNjZoOVJsNG0xdFJjCnViZ1o2dWZCZ3hGVDlXS21mSEdCNm4zelh5b2pQOEFJTnR6ODVpaUVHVXFFRWtVRVdMd0RGSmdvcFllQ3l1VmZ2ZE92MUgKRm1WWmEwNVo0b3NQNkNENXVmc2djQ1RYQTR6VnZ5ZHVCYkxqdHN5RjdYZjNUdjZUQ1QxU0swZHErQk1OOXRvb0RZaXpwagpzbDh6NzlybXp3eUFyWFlVcnFUUkpsNmpwRkNrWHJLcy9LeG96MHhhbXlMY2RORk9hWE51LzlnTkpjRERsV2hPcFRqNHk4CkpkNXBuV1Jueis1RHJLRFdhY0loUW1CMUxVd2ZLWmQwbVFxaUpzMUMxcXZVUmlKOGExaThKUTI4bHFuWTFRRk9wbk13emcKWEpla2FndThpT1ExRFJlQkhaM0NkcVJUYnY3bVJZSGxramx0dXJmZGc4M3hvM0ErZ1JSR001eUVOcW5xSkplQjhJQVB5UwptMFp0dGdqbHNqNTJ2K1B1NmExMHoxZndKK1VML2N6dTRKeEpOYlp6WTFIMnpLODJBaVI1T3JYNmx2aUEvSWFSRVcwUUFBCkFNQndVeUJpcUc5bEZCUnltL2UvU1VORVMzdHpicUZNdTdIcy84WTV5SnAxKzR6OXUxNGtJR2ttV0Y5eE5HT3hrY3V0cWwKeHVUcndMbjFUaFNQTHQrTjUwTGhVdzR4ZjBhNUxqemdPbklPU0FRbm5HY1Nxa0dTRDlMR21obGE2WmpydFBHY29lQ3JHdAo5M1Vvcmx5YkxNRzFFRFAxWmpKS1RaZzl6OUMwdDlTTGd3ei9DbFhydW9UNXNQVUdKWnUrbHlIZXpSTDRtcHl6OEZMcnlOCkdNci9leVM5bWdISjNVVkZEYjNIZ3BaK1E1SUdBRU5rZVZEcHIwMGhCZXZndGd6YWtBQUFEQkFQVXQ1RitoMnBVby94V1YKenRkcVQvMzA4dFB5MXVMMU1lWFoydEJPQmRwSDJyd0JzdWt0aTIySGtWZUZXQjJFdUlFUXppMzY3MGc1UGdxR1p4Vng4dQpobEE0Rkg4ZXN1NTNQckZqVW9EeFJhb3d3WXBFcFh5Y2pnNUE1MStwR1VQcWljWjB0YjliaWlhc3BWWXZhWW5sdGlnVG5iClN0UExMY29nemNiL0dGcVYyaXlzc3lwTlMwKzBNRTUxcEtxWGNaS2swbi8vVHpZWWs4TW8vZzRsQ3pmUEZQUlZrVVM5blIKWU1pQzRlcEk0TERmbVdnM0xLQ2N1Zk85all3aWgwYlFBQUFNRUE2WEtldDhEMHNvc0puZVh5WFZGd0dyVyszNlhBVGRQTwpMWDdjaStjYzFoOGV1eHdYQWx3aTJJNFhxSmJBVjBsVEhuVGEycXN3Uy9RQlpJUUJWSkZlVjVyS1daZTc4R2F3d1pWTFZNCldETmNwdFFyRTFaM2pGNS9TdUVzdlVxSDE0Tkc5RUFXWG1iUkNzelE0Vlk3NzQrSi9sTFkvMnlDT1diNzlLYTJ5OGxvYUoKVXczWWVtSld3blp2R3hKNldsL3BmQ2xYN3lEVXlXUktLdGl0cWNjbmpCWVkyRE1tZURwdURDYy9ZdDZDc3dLRmRkMkJ1UwpGZGt5cDlZY3VMaDlLZEFBQUFIR3BoYzI5dVFFRlVMVGd3TWxVdWJXOXVkR3hsYjI0dWFXNTBjbUVCQWdNRUJRWT0KLS0tLS1FTkQgT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0tCgo=
kind: Secret
metadata:
  name: ssh-credentials
  namespace: konveyor-forklift
type: Opaque
```

### Create the Hook

```
apiVersion: forklift.konveyor.io/v1beta1
kind: Hook
metadata:
  name: playbook
  namespace: konveyor-forklift
spec:
  image: quay.io/konveyor/hook-runner
  playbook: LSBuYW1lOiBNYWluCiAgaG9zdHM6IGxvY2FsaG9zdAogIHRhc2tzOgogIC0gbmFtZTogTG9hZCBQbGFuCiAgICBpbmNsdWRlX3ZhcnM6CiAgICAgIGZpbGU6IHBsYW4ueW1sCiAgICAgIG5hbWU6IHBsYW4KCiAgLSBuYW1lOiBMb2FkIFdvcmtsb2FkCiAgICBpbmNsdWRlX3ZhcnM6CiAgICAgIGZpbGU6IHdvcmtsb2FkLnltbAogICAgICBuYW1lOiB3b3JrbG9hZAoKICAtIG5hbWU6IAogICAgZ2V0ZW50OgogICAgICBkYXRhYmFzZTogcGFzc3dkCiAgICAgIGtleTogInt7IGFuc2libGVfdXNlcl9pZCB9fSIKICAgICAgc3BsaXQ6ICc6JwoKICAtIG5hbWU6IEVuc3VyZSBTU0ggZGlyZWN0b3J5IGV4aXN0cwogICAgZmlsZToKICAgICAgcGF0aDogfi8uc3NoCiAgICAgIHN0YXRlOiBkaXJlY3RvcnkKICAgICAgbW9kZTogMDc1MAogICAgZW52aXJvbm1lbnQ6CiAgICAgIEhPTUU6ICJ7eyBhbnNpYmxlX2ZhY3RzLmdldGVudF9wYXNzd2RbYW5zaWJsZV91c2VyX2lkXVs0XSB9fSIKCiAgLSBrOHNfaW5mbzoKICAgICAgYXBpX3ZlcnNpb246IHYxCiAgICAgIGtpbmQ6IFNlY3JldAogICAgICBuYW1lOiBzc2gtY3JlZGVudGlhbHMKICAgICAgbmFtZXNwYWNlOiBrb252ZXlvci1mb3JrbGlmdAogICAgcmVnaXN0ZXI6IHNzaF9jcmVkZW50aWFscwoKICAtIG5hbWU6IENyZWF0ZSBTU0gga2V5CiAgICBjb3B5OgogICAgICBkZXN0OiB+Ly5zc2gvaWRfcnNhCiAgICAgIGNvbnRlbnQ6ICJ7eyBzc2hfY3JlZGVudGlhbHMucmVzb3VyY2VzWzBdLmRhdGEua2V5IHwgYjY0ZGVjb2RlIH19IgogICAgICBtb2RlOiAwNjAwCgogIC0gYWRkX2hvc3Q6CiAgICAgIG5hbWU6ICJ7eyB3b3JrbG9hZC52bS5pcGFkZHJlc3MgfX0iCiAgICAgIGFuc2libGVfdXNlcjogcm9vdAogICAgICBncm91cHM6IHZtcwoKLSBob3N0czogdm1zCiAgdGFza3M6CiAgLSBuYW1lOiBJbnN0YWxsIGNsb3VkLWluaXQKICAgIGRuZjoKICAgICAgbmFtZToKICAgICAgLSBjbG91ZC1pbml0CiAgICAgIHN0YXRlOiBsYXRlc3QKCiAgLSBuYW1lOiBDcmVhdGUgVGVzdCBGaWxlCiAgICBjb3B5OgogICAgICBkZXN0OiAvdGVzdC50eHQKICAgICAgY29udGVudDogIkhlbGxvIFdvcmxkIgogICAgICBtb2RlOiAwNjQ0Cg==
  serviceAccount: forklift-controller
```

The playbook encoded here does the following:
```
- name: Main
  hosts: localhost
  tasks:
  - name: Load Plan
    include_vars:
      file: plan.yml
      name: plan

  - name: Load Workload
    include_vars:
      file: workload.yml
      name: workload

  - name: 
    getent:
      database: passwd
      key: "{{ ansible_user_id }}"
      split: ':'

  - name: Ensure SSH directory exists
    file:
      path: ~/.ssh
      state: directory
      mode: 0750
    environment:
      HOME: "{{ ansible_facts.getent_passwd[ansible_user_id][4] }}"

  - k8s_info:
      api_version: v1
      kind: Secret
      name: ssh-credentials
      namespace: konveyor-forklift
    register: ssh_credentials

  - name: Create SSH key
    copy:
      dest: ~/.ssh/id_rsa
      content: "{{ ssh_credentials.resources[0].data.key | b64decode }}"
      mode: 0600

  - add_host:
      name: "{{ workload.vm.ipaddress }}"
      ansible_user: root
      groups: vms

- hosts: vms
  tasks:
  - name: Install cloud-init
    dnf:
      name:
      - cloud-init
      state: latest

  - name: Create Test File
    copy:
      dest: /test.txt
      content: "Hello World"
      mode: 0644
```

### Create the plan using the hook

```
kind: Plan
apiVersion: forklift.konveyor.io/v1beta1
metadata:
  name: test
  namespace: konveyor-forklift
spec:
  map:
    network:
      namespace: "konveyor-forklift"
      name: "network"
    storage:
      namespace: "konveyor-forklift"
      name: "storage"
  provider:
    source:
      namespace: "konveyor-forklift"
      name: "boston"
    destination:
      namespace: "konveyor-forklift"
      name: host
  targetNamespace: "konveyor-forklift"
  vms:
    - id: vm-2861
      hooks:
        - hook:
            namespace: konveyor-forklift
            name: playbook
          step: PreHook
```
