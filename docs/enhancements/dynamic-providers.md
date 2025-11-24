# Dynamic Providers

**Version:** v2.11 
**Status:** Proposed

---

## Problem

Adding new provider types to Forklift requires modifying controller code, rebuilding, and redeploying the entire system.

---

## Solution

Enable dynamic provider types through HTTP-based provider servers managed by two new CRDs:

1. **DynamicProvider** - Defines a provider type once (e.g., "ova")
2. **DynamicProviderServer** - Manages server instances (one per Provider)

---

## Architecture

```
┌─────────────────┐
│ DynamicProvider │  Type definition (one per type)
│ - Type: "ova"   │  - Container image
│ - Image         │  - Capabilities
│ - Features      │  - Default resources
└────────┬────────┘
         │ references
         ↓
┌──────────────────────┐      ┌─────────────┐
│DynamicProviderServer │◄─────│  Provider   │
│ - DynamicProviderRef │ 1:1  │  Type: ova  │
│ - ProviderRef        │      │  URL: ...   │
│ - Pod + Service      │      └─────────────┘
└──────────────────────┘
```

**Relationships:**
- 1 DynamicProvider → N DynamicProviderServers
- 1 Provider ↔ 1 DynamicProviderServer (1:1)

---

## CRDs

### DynamicProvider

Defines a provider type and its capabilities.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: DynamicProvider
metadata:
  name: ova-provider
spec:
  type: ova                                        # Required: Type identifier
  image: quay.io/konveyor/ova-provider:latest      # Required: Server image
  port: 8080                                       # Optional: Server port
  
  features:                                        # Optional: Capabilities
    requiresConversion: true
    supportsCustomBuilder: true
    supportedMigrationTypes: [cold]
  
  storages:                                        # Optional: Working space PVCs
    - name: workspace
      size: 50Gi
      mountPath: /workspace
  
  resources:                                       # Optional: Resource limits
    requests:
      memory: 256Mi
      cpu: 100m
```

### DynamicProviderServer

Manages a server instance for a specific Provider.

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: DynamicProviderServer
metadata:
  name: my-ova-server
spec:
  dynamicProviderRef:           # Required: Links to DynamicProvider
    name: ova-provider
  
  providerRef:                  # Required: Links to Provider (1:1)
    name: my-ova-provider
  
  storages:                     # Optional: From DynamicProvider (creates PVCs)
    - name: workspace
      size: 50Gi
      mountPath: /workspace
  
  volumes:                      # Optional: From Provider (existing sources only)
    - name: ova-files
      mountPath: /ova
      source:
        nfs:
          server: nas.example.com
          path: /exports/ova
```

### Provider (Enhanced)

Standard Provider CR with new optional field:

```yaml
apiVersion: forklift.konveyor.io/v1beta1
kind: Provider
metadata:
  name: my-ova-provider
spec:
  type: ova                     # Must match DynamicProvider.spec.type
  url: nfs://nas.example.com/ova
  
  secret:                       # Optional: Credentials for authentication
    name: my-provider-creds      # If specified, mounted to server pod
    namespace: openshift-mtv     # at /etc/forklift/credentials
  
  volumes:                      # Optional: Existing volumes to mount (not created)
    - name: ova-files
      mountPath: /ova
      source:
        nfs:
          server: nas.example.com
          path: /exports/ova
  
  serverNodeSelector:           # Optional: Schedule server on specific nodes
    disktype: ssd
    zone: us-east-1a
  
  serverAffinity:               # Optional: Affinity rules for server pod
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: node.kubernetes.io/instance-type
                operator: In
                values:
                  - m5.xlarge
                  - m5.2xlarge
```

---

## Workflow

1. **Admin** creates `DynamicProvider` defining the provider type
2. **User** creates `Provider` with matching type (optionally with secret)
3. **Controller** validates `Provider`:
   - For dynamic providers: Validates secret exists (if specified)
   - Does NOT validate secret contents
4. **Operator** auto-creates `DynamicProviderServer`:
   - Copies `storages` from DynamicProvider
   - Copies `volumes` from Provider
   - Mounts `secret` from Provider (if specified)
5. **Server Controller** creates:
   - PVCs (from storages)
   - Deployment (with all volumes and secret mount)
   - Service
6. **Provider Server** starts:
   - Reads credentials from `/etc/forklift/credentials` (if mounted)
   - Validates authentication during `/test_connection` call
7. **Forklift** uses server for inventory and migrations

---

## API Endpoints

Provider servers must implement these HTTP endpoints:

### Required Endpoints

#### Connection and Health

```
GET  /test_connection                            → Health check and connection test
                                                    - Called during provider validation
                                                    - Must return HTTP 200 for success
                                                    - Used to verify service availability
```

#### Inventory Collection

```
GET  /vms                                        → List VMs
GET  /networks                                   → List networks  
GET  /storages                                   → List storage resources
GET  /disks                                      → List disks
```

#### Migration Operations

```
POST /vms/{id}/disks/{diskId}/datavolume-source  → Get disk source for data transfer using CDI data volumes
                                                    - Required for VM migration
                                                    - Returns CDI DataVolume source specification
```

### Optional Endpoints

#### Version Information

```
GET  /version                                    → Get provider server version
                                                    - Returns: major, minor, build, revision
                                                    - Optional: won't fail if unavailable
```

#### Custom Builder (requires `supportsCustomBuilder: true`)

```
POST /vms/{vmId}/build-spec                      → Build VirtualMachine spec
                                                    - Output: KubeVirt VirtualMachine JSON
                                                    - Receives plan reference in request body
                                                    - Provider queries cluster for full context
```

#### Conversion Support

```
GET  /vms/{id}/v2v-input-type                    → Get virt-v2v input mode
                                                    - Returns: "ova" or "libvirtxml"
                                                    - Used when guest conversion is needed

GET  /vms/{id}/libvirtxml                        → Get libvirt domain XML
                                                    - Required if v2v-input-type returns "libvirtxml"
                                                    - Used for conversion process
```

### Custom Builder

When `supportsCustomBuilder: true`, the `/build-spec` endpoint receives:

**Request:** Plan reference
```json
{
  "plan": {
    "name": "migration-plan",
    "namespace": "openshift-mtv"
  }
}
```

Provider server queries cluster for Plan CR, mappings, PVCs, and uses local inventory.

---

### Validation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. User creates Provider CR                                     │
│    - With or without secret reference                           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Controller validates Provider                                │
│    - Dynamic provider: Only checks secret EXISTS (if specified) │
│    - Static provider: Validates secret contents                 │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. DynamicProviderServer created                                │
│    - Secret mounted (if exists) to /etc/forklift/credentials    │
│    - PROVIDER_CREDENTIALS_PATH env var set                      │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Provider server validates authentication                     │
│    - Reads credentials from mounted path                        │
│    - Validates during /test_connection call                     │
│    - Returns success/failure to controller                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Volume Management

Two optional volume types:

### 1. Storages (Dynamic Working Space)
- **Source:** `DynamicProvider.spec.storages`
- **Purpose:** Temporary files, cache, processing
- **Managed by:** Controller (creates PVCs with owner references)
- **Example:** 50Gi PVC for extracting OVA files
- **Lifecycle:** Created/deleted with DynamicProviderServer

### 2. Volumes (Existing Data Sources)
- **Source:** `Provider.spec.volumes`
- **Purpose:** Mount existing data sources (NFS shares, ConfigMaps, existing PVCs)
- **Managed by:** User (must already exist or be inline definitions)
- **Example:** NFS mount with OVA files, ConfigMap with config
- **Lifecycle:** External - NOT created or deleted by controller
- **Important:** These use standard Kubernetes VolumeSource - embedded directly in Pod spec

**Both are optional.** Use based on provider needs:
- Stateless API provider: Neither
- Read-only scanner: Only volumes (for existing data)
- File processor: Both (storages for work space + volumes for input data)

---

## Benefits

- **No Controller Changes** - Add providers without code modifications  
- **Independent Scaling** - Scale each provider separately  
- **Resource Isolation** - Dedicated resources per provider  
- **Flexible Storage** - Optional working space and external data  

---

## Implementation

### Controllers

**DynamicProvider Controller:**
- Validates configuration
- Tracks active servers

**DynamicProviderServer Controller:**
- Creates PVCs from storages
- Creates Deployment with volumes
- Creates Service
- Updates status

**Provider Controller:**
- Auto-creates DynamicProviderServer for dynamic types
- Copies storages and volumes to server

### Volume Populator Support

Providers can return volume populator references in datavolume-source responses for custom data transfer logic.

---

## Scheduling Options

Users can control where provider server pods run:

### Node Selector

Schedule server on nodes with specific labels:

```yaml
spec:
  serverNodeSelector:
    disktype: ssd
    workload: provider-servers
```

### Affinity

Advanced scheduling rules:

```yaml
spec:
  serverAffinity:
    # Node affinity - prefer specific node types
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          preference:
            matchExpressions:
              - key: node-role.kubernetes.io/worker
                operator: In
                values: [provider-node]
    
    # Pod anti-affinity - spread providers across nodes
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app: provider-server
            topologyKey: kubernetes.io/hostname
```
