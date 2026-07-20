# Mock Nutanix Prism API Server

A simple HTTPS server that simulates the Nutanix Prism v3 API for testing purposes.

## Quick Start

```bash
# From the testdata directory
cd pkg/controller/provider/container/nutanix/testdata

# Generate self-signed certificate (first time only)
openssl req -x509 -newkey rsa:4096 -keyout server-key.pem -out server-cert.pem \
  -days 365 -nodes -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

# Run the mock server
go run mock_server.go

# Or with custom settings
go run mock_server.go -port 9440 -user admin -password mypassword \
  -cert server-cert.pem -key server-key.pem
```

The server will start on https://localhost:9440 by default.

## Testing with curl

```bash
# Test authentication and list clusters
curl -k -X POST https://localhost:9440/api/nutanix/v3/clusters/list \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"kind": "cluster"}'

# List VMs
curl -k -X POST https://localhost:9440/api/nutanix/v3/vms/list \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{"kind": "vm", "length": 100, "offset": 0}'

# List hosts
curl -k -X POST https://localhost:9440/api/nutanix/v3/hosts/list \
  -u admin:password \
  -H "Content-Type: application/json" \
  -d '{}'
```

Note: The `-k` flag is needed to skip certificate verification for self-signed certificates.

## Testing with Forklift

1. **Start the mock server:**
   ```bash
   go run mock_server.go
   ```

2. **Create a Kubernetes Secret:**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: nutanix-provider-credentials
     namespace: konveyor-forklift
   type: Opaque
   stringData:
     user: admin
     password: password
     insecureSkipVerify: "true"  # Required for localhost testing
   ```

3. **Create a Nutanix Provider CR:**
   ```yaml
   apiVersion: forklift.konveyor.io/v1beta1
   kind: Provider
   metadata:
     name: nutanix-mock
     namespace: konveyor-forklift
   spec:
     type: nutanix
     url: http://localhost:9440
     secret:
       name: nutanix-provider-credentials
       namespace: konveyor-forklift
   ```

4. **Check the provider status:**
   ```bash
   kubectl get provider nutanix-mock -n konveyor-forklift -o yaml
   ```

## Available Endpoints

All endpoints require HTTP Basic Authentication.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/nutanix/v3` | GET/POST | API version info |
| `/api/nutanix/v3/clusters/list` | POST | List clusters (2 clusters) |
| `/api/nutanix/v3/hosts/list` | POST | List hosts (3 hosts) |
| `/api/nutanix/v3/vms/list` | POST | List VMs (6 VMs) |
| `/api/nutanix/v3/subnets/list` | POST | List networks (3 subnets) |
| `/api/nutanix/v3/storage_containers/list` | POST | List storage (2 containers) |
| `/api/nutanix/v3/images/list` | POST | List images (3 images) |

## Command Line Options

```
-port string
    Port to listen on (default: "9440")
-user string
    Username for basic auth (default: "admin")
-password string
    Password for basic auth (default: "password")
```

## Test Data

The mock server serves the JSON files in this directory:
- `clusters_list.json` - 2 Nutanix clusters
- `hosts_list.json` - 3 AHV hosts
- `vms_list.json` - 6 virtual machines with full details
- `subnets_list.json` - 3 network subnets
- `storage_containers_list.json` - 2 storage containers
- `images_list.json` - 3 disk images

## API Behavior

The mock server simulates Nutanix Prism API v3 behavior:
- **Authentication**: HTTP Basic Auth required on all endpoints
- **Method**: POST requests for all list operations (not GET)
- **Content-Type**: `application/json`
- **Pagination**: Request body can include `length` and `offset` (currently ignored, returns all data)
- **Filtering**: Request body can include filters (currently ignored)

## Limitations

This is a simple mock server for testing. It does not implement:
- Pagination (always returns all results)
- Filtering (ignores filter criteria in request body)
- Individual resource GET operations
- Resource creation/update/delete
- Real Nutanix API error responses
- Rate limiting
- SSL/TLS (use insecureSkipVerify in testing)

## Use Cases

- **Unit Testing**: Test Nutanix provider code without a real cluster
- **Integration Testing**: Test full provider integration in Forklift
- **Development**: Develop Nutanix provider features without infrastructure
- **CI/CD**: Automated testing in pipelines
- **Demos**: Demonstrate Nutanix provider without real infrastructure
