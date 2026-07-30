//go:build integration

package nutanix

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	webnutanix "github.com/kubev2v/forklift/pkg/controller/provider/web/nutanix"
)

// TestDomainXMLFromInventory generates and prints domain XML for a real Nutanix VM
// fetched from a running forklift inventory.
//
// Required env vars:
//
//	INVENTORY_URL    base URL of the forklift inventory service
//	                 e.g. https://localhost:8443  (after kubectl port-forward)
//	PROVIDER_UID     UID of the Nutanix Provider CR
//	VM_NAME          name of the VM to generate XML for (lists all VMs if omitted)
func TestDomainXMLFromInventory(t *testing.T) {
	inventoryURL := os.Getenv("INVENTORY_URL")
	providerUID := os.Getenv("PROVIDER_UID")
	if inventoryURL == "" || providerUID == "" {
		t.Skip("INVENTORY_URL and PROVIDER_UID must be set to run this test")
	}

	token := os.Getenv("AUTH_TOKEN")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}

	do := func(url string) (*http.Response, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return client.Do(req)
	}

	vmName := os.Getenv("VM_NAME")

	// Fetch VM list or a specific VM by name.
	vmsURL := fmt.Sprintf("%s/providers/nutanix/%s/vms", inventoryURL, providerUID)
	resp, err := do(vmsURL)
	if err != nil {
		t.Fatalf("GET %s: %v", vmsURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: status %d: %s", vmsURL, resp.StatusCode, body)
	}

	var vms []webnutanix.VM
	if err := json.NewDecoder(resp.Body).Decode(&vms); err != nil {
		t.Fatalf("decoding VM list: %v", err)
	}

	if len(vms) == 0 {
		t.Fatal("no VMs found for this provider")
	}

	if vmName == "" {
		t.Logf("VM_NAME not set; available VMs:")
		for _, vm := range vms {
			t.Logf("  %-40s  %s", vm.Name, vm.ID)
		}
		t.Log("Set VM_NAME to generate domain XML for a specific VM.")
		return
	}

	// Find the requested VM.
	var found *webnutanix.VM
	for i := range vms {
		if vms[i].Name == vmName {
			found = &vms[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("VM %q not found; run without VM_NAME to list available VMs", vmName)
	}

	// Fetch the full VM detail (detail level 2 includes all fields).
	vmURL := fmt.Sprintf("%s/providers/nutanix/%s/vms/%s", inventoryURL, providerUID, found.ID)
	resp2, err := do(vmURL)
	if err != nil {
		t.Fatalf("GET %s: %v", vmURL, err)
	}
	defer resp2.Body.Close()
	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading VM detail: %v", err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", vmURL, resp2.StatusCode, body)
	}

	var fullVM webnutanix.VM
	if err := json.Unmarshal(body, &fullVM); err != nil {
		t.Fatalf("decoding VM detail: %v", err)
	}

	// Convert webnutanix.VM → model.VM (the fields are the same; NICs/Disks are type aliases).
	vm := &model.VM{
		Base:              model.Base{ID: fullVM.ID, Name: fullVM.Name},
		UUID:              fullVM.UUID,
		PowerState:        fullVM.PowerState,
		Description:       fullVM.Description,
		NumSockets:        fullVM.NumSockets,
		NumVcpusPerSocket: fullVM.NumVcpusPerSocket,
		NumThreadsPerCore: fullVM.NumThreadsPerCore,
		MemorySizeMiB:     fullVM.MemorySizeMiB,
		BootType:          fullVM.BootType,
		BootDeviceOrder:   fullVM.BootDeviceOrder,
		MachineType:       fullVM.MachineType,
		HardwareClockTZ:   fullVM.HardwareClockTZ,
		GuestOSID:         fullVM.GuestOSID,
		NICs:              fullVM.NICs,
		Disks:             fullVM.Disks,
		SerialPorts:       fullVM.SerialPorts,
	}

	t.Logf("VM: %s (%s)", vm.Name, vm.UUID)
	t.Logf("  BootType: %s  MachineType: %s", vm.BootType, vm.MachineType)
	t.Logf("  Disks: %d  NICs: %d", len(vm.Disks), len(vm.NICs))

	// Generate domain XML with no PVCs (placeholder filesystem paths will be used).
	xmlStr, err := buildDomainXML(vm, nil)
	if err != nil {
		t.Fatalf("buildDomainXML: %v", err)
	}

	fmt.Println("\n=== Generated libvirt domain XML ===")
	fmt.Println(xmlStr)
	fmt.Println("====================================")
}
