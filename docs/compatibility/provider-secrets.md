# Provider Secrets Reference

| Metadata | Value |
|----------|-------|
| **Last Updated** | January 22, 2026 |
| **Applies To** | Forklift v2.11 |
| **Maintainer** | Forklift Team |

This document details the secret fields required for each provider type. Secrets are referenced in the Provider CR via `spec.secret`.

## Common Fields

These fields are available across multiple providers:

| Field | Type | Description |
|-------|------|-------------|
| `insecureSkipVerify` | string | Set to `"true"` to skip TLS certificate verification (not recommended for production) |
| `cacert` | string | CA certificate in PEM format for TLS verification |

---

## VMware vSphere

Authentication to vCenter or ESXi hosts.

### Required Fields

| Field | Description |
|-------|-------------|
| `user` | vCenter/ESXi username (e.g., `administrator@vsphere.local`) |
| `password` | vCenter/ESXi password |

### Optional Fields

| Field | Description |
|-------|-------------|
| `cacert` | vCenter CA certificate in PEM format |
| `insecureSkipVerify` | Skip TLS verification (`"true"` or `"false"`) |
| `thumbprint` | vCenter SSL thumbprint (alternative to cacert) |

### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  user: administrator@vsphere.local
  password: "your-password"
  cacert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

---

## Red Hat Virtualization (oVirt)

Authentication to oVirt/RHV Manager.

### Required Fields

| Field | Description |
|-------|-------------|
| `user` | oVirt username (e.g., `admin@internal`) |
| `password` | oVirt password |

### Optional Fields

| Field | Description |
|-------|-------------|
| `cacert` | oVirt Manager CA certificate in PEM format |
| `insecureSkipVerify` | Skip TLS verification (`"true"` or `"false"`) |

### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ovirt-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  user: admin@internal
  password: "your-password"
  cacert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

---

## OpenStack

Authentication to OpenStack Keystone. Supports multiple authentication methods.

### Authentication Types

Set the `authType` field to one of:
- `password` (default) - Username/password authentication
- `token` - Token-based authentication
- `applicationcredential` - Application credential authentication

### Password Authentication

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | OpenStack username |
| `password` | Yes | OpenStack password |
| `projectName` | Yes* | Project/tenant name |
| `projectID` | Yes* | Project/tenant ID (alternative to projectName) |
| `userDomainName` | Yes* | User domain name |
| `userDomainID` | Yes* | User domain ID (alternative to userDomainName) |
| `projectDomainName` | No | Project domain name |
| `projectDomainID` | No | Project domain ID |
| `domainName` | No | Domain name (for domain-scoped tokens) |
| `domainID` | No | Domain ID |
| `defaultDomain` | No | Default domain |
| `regionName` | No | OpenStack region |

*One of each pair required

### Application Credential Authentication

| Field | Required | Description |
|-------|----------|-------------|
| `applicationCredentialID` | Yes* | Application credential ID |
| `applicationCredentialName` | Yes* | Application credential name (requires userID or username) |
| `applicationCredentialSecret` | Yes | Application credential secret |
| `userID` | Cond. | User ID (required with applicationCredentialName) |

### Common Optional Fields

| Field | Description |
|-------|-------------|
| `cacert` | Keystone CA certificate in PEM format |
| `insecureSkipVerify` | Skip TLS verification |
| `availability` | Endpoint availability (`public`, `internal`, `admin`) |

### Example (Password Auth)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openstack-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  authType: password
  username: admin
  password: "your-password"
  projectName: admin
  userDomainName: Default
  regionName: RegionOne
```

---

## OpenShift Virtualization

Authentication to remote OpenShift clusters for cross-cluster migration.

### Required Fields

| Field | Description |
|-------|-------------|
| `token` | OpenShift service account token or user token |

### Optional Fields

| Field | Description |
|-------|-------------|
| `cacert` | OpenShift API CA certificate |
| `insecureSkipVerify` | Skip TLS verification |

### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openshift-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  token: "sha256~xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

**Note:** For the "host" provider (same cluster), no secret is required.

---

## OVA

OVA providers use NFS endpoints to access OVA files. Authentication depends on the storage backend.

### NFS-based OVA

No secret fields required - access is based on NFS permissions.

### HTTP-based OVA (if applicable)

| Field | Description |
|-------|-------------|
| `username` | HTTP basic auth username (if required) |
| `password` | HTTP basic auth password (if required) |

### Example (NFS)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ova-credentials
  namespace: openshift-mtv
type: Opaque
stringData: {}
```

The provider URL specifies the NFS endpoint: `nfs://server/path/to/ova`

---

## Amazon EC2

Authentication to AWS for EC2 instance migration.

### Required Fields

| Field | Description |
|-------|-------------|
| `region` | AWS region where source EC2 instances are located (e.g., `us-east-1`) |
| `accessKeyId` | AWS access key ID for source account |
| `secretAccessKey` | AWS secret access key for source account |

### Optional Fields (Cross-Account Migration)

| Field | Description |
|-------|-------------|
| `targetAccessKeyId` | AWS access key ID for target account |
| `targetSecretAccessKey` | AWS secret access key for target account |

**Note:** For same-account migrations, use the OpenShift cluster's AWS credentials to ensure the EBS CSI driver can access the migrated volumes.

### Example (Same-Account)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ec2-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  region: us-east-1
  accessKeyId: AKIAIOSFODNN7EXAMPLE
  secretAccessKey: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

### Example (Cross-Account)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ec2-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  region: us-east-1
  accessKeyId: AKIAIOSFODNN7EXAMPLE
  secretAccessKey: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  targetAccessKeyId: AKIAI44QH8DHBEXAMPLE
  targetSecretAccessKey: je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY
```

---

## Hyper-V

Authentication to Microsoft Hyper-V servers. Uses similar structure to OVA providers.

### Required Fields

| Field | Description |
|-------|-------------|
| `username` | Hyper-V server username |
| `password` | Hyper-V server password |

### Optional Fields

| Field | Description |
|-------|-------------|
| `cacert` | Server CA certificate |
| `insecureSkipVerify` | Skip TLS verification |

### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hyperv-credentials
  namespace: openshift-mtv
type: Opaque
stringData:
  username: administrator
  password: "your-password"
```

---

## Summary Table

| Field | vSphere | oVirt | OpenStack | OpenShift | OVA | EC2 | HyperV |
|-------|:-------:|:-----:|:---------:|:---------:|:---:|:---:|:------:|
| `user` / `username` | Req | Req | Req* | - | Opt | - | Req |
| `password` | Req | Req | Req* | - | Opt | - | Req |
| `token` | - | - | Opt | Req | - | - | - |
| `cacert` | Opt | Opt | Opt | Opt | - | - | Opt |
| `insecureSkipVerify` | Opt | Opt | Opt | Opt | - | - | Opt |
| `region` | - | - | Opt | - | - | Req | - |
| `accessKeyId` | - | - | - | - | - | Req | - |
| `secretAccessKey` | - | - | - | - | - | Req | - |
| `projectName` | - | - | Req* | - | - | - | - |
| `userDomainName` | - | - | Req* | - | - | - | - |
| `applicationCredentialID` | - | - | Opt | - | - | - | - |

**Legend:** Req = Required, Opt = Optional, Req* = Required for specific auth type, - = Not applicable
