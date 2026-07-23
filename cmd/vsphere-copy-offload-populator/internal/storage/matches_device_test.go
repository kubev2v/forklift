package storage_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/flashsystem"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/infinibox"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/ontap"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/powerflex"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/powermax"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/powerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/primera3par"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/pure"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/storage"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vantara"
)

// Real NAA values from production arrays, used to validate serial/WWN extraction.
// These were obtained by querying each array's API during development.
const (
	// ONTAP: naa.600a0980 + hex(serial). Serial "819TI$X101LA" → hex "383139544924583130314c41"
	ontapRealNAA = "naa.600a0980383139544924583130314c41"

	// 3PAR: naa. + lowercase(WWN). WWN already starts with 60002AC...
	par3RealNAA = "naa.60002ac0000000000000182d00021f6b"

	// Pure: naa.624a9370 + lowercase(serial). Serial "A7B9F7ECC01E40F70001181F"
	pureRealNAA = "naa.624a9370a7b9f7ecc01e40f70001181f"

	// PowerFlex: eui.{systemId}{volumeId}. SystemId "b4f2d5322f73780f"
	powerflexRealEUI = "eui.b4f2d5322f73780f5a5beec600000002"

	// PowerStore: naa.68ccf098 + rest of WWN
	powerstoreRealNAA = "naa.68ccf098001b37dba855c0a4eabe6ab6"

	// Infinibox: naa.6 + serial. Serial "742b0f000000f6f00000000000102a1"
	infiniboxRealNAA = "naa.6742b0f000000f6f00000000000102a1"

	// FlashSystem: naa. + vdisk_UID (UID starts with 6005076...)
	flashsystemRealNAA = "naa.6005076400CE800080000000000004B6"

	// PowerMax: naa.60000970 + rest of WWN
	powermaxRealNAA = "naa.60000970000297700461533030333846"

	// Vantara: naa.60060e80 + 32 hex chars total; last 4 hex chars are the LDEV ID.
	// "0d00" → decimal 3328.
	vantaraRealNAA = "naa.60060e80233abc0050703abc00000d00"
)

// TestMatchesDevice_PrefixReject verifies that each provider fast-rejects
// devices from different vendors without making API calls.
func TestMatchesDevice_PrefixReject(t *testing.T) {
	tests := []struct {
		name       string
		provider   storage.ArrayIdentifier
		deviceName string
	}{
		// ONTAP rejects non-ONTAP
		{"ONTAP rejects 3PAR", &ontap.NetappClonner{}, par3RealNAA},
		{"ONTAP rejects Pure", &ontap.NetappClonner{}, pureRealNAA},
		{"ONTAP rejects PowerFlex", &ontap.NetappClonner{}, powerflexRealEUI},
		{"ONTAP rejects local", &ontap.NetappClonner{}, "mpx.vmhba0:C0:T1:L0"},

		// 3PAR rejects non-3PAR
		{"3PAR rejects ONTAP", &primera3par.Primera3ParClonner{}, ontapRealNAA},
		{"3PAR rejects PowerFlex", &primera3par.Primera3ParClonner{}, powerflexRealEUI},

		// PowerMax rejects non-PowerMax
		{"PowerMax rejects ONTAP", &powermax.PowermaxClonner{}, ontapRealNAA},
		{"PowerMax rejects 3PAR", &powermax.PowermaxClonner{}, par3RealNAA},
		{"PowerMax rejects PowerFlex", &powermax.PowermaxClonner{}, powerflexRealEUI},

		// PowerFlex rejects non-PowerFlex (empty systemId)
		{"PowerFlex rejects ONTAP", &powerflex.PowerflexClonner{}, ontapRealNAA},
		{"PowerFlex rejects 3PAR", &powerflex.PowerflexClonner{}, par3RealNAA},
		{"PowerFlex rejects Pure", &powerflex.PowerflexClonner{}, pureRealNAA},
		{"PowerFlex rejects local", &powerflex.PowerflexClonner{}, "naa.55cd2e414d53564f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.provider.MatchesDevice(tt.deviceName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got {
				t.Errorf("MatchesDevice(%q) = true, want false (wrong vendor)", tt.deviceName)
			}
		})
	}
}

// TestMatchesDevice_SerialExtraction validates that the serial/WWN/UID
// is correctly extracted from the NAA for providers that don't need an
// API call (PowerFlex) or that use prefix-only (PowerMax).
func TestMatchesDevice_SerialExtraction_PowerFlex(t *testing.T) {
	// PowerFlex with matching systemId → should match
	pf := powerflex.NewPowerflexClonnerForTest("b4f2d5322f73780f")
	got, err := pf.MatchesDevice(powerflexRealEUI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true (same systemId)", powerflexRealEUI)
	}

	// Different systemId → should not match
	pfOther := powerflex.NewPowerflexClonnerForTest("aaaaaaaaaaaaaaaa")
	got, err = pfOther.MatchesDevice(powerflexRealEUI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Errorf("MatchesDevice(%q) = true, want false (different systemId)", powerflexRealEUI)
	}
}

// TestMatchesDevice_ONTAPSerialDecode verifies the hex→ASCII serial decode
// from the NAA matches what the ONTAP API returns.
func TestMatchesDevice_ONTAPSerialDecode(t *testing.T) {
	wantSerial := "819TI$X101LA"
	var gotSerial string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSerial = r.URL.Query().Get("serial_number")
		json.NewEncoder(w).Encode(map[string]interface{}{"num_records": 1})
	}))
	defer server.Close()

	c := ontap.NewNetappClonnerForTest(server.Listener.Addr().String(), "test-svm")
	got, err := c.MatchesDevice(ontapRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", ontapRealNAA)
	}
	if gotSerial != wantSerial {
		t.Errorf("serial sent to array = %q, want %q (serial not decoded correctly)", gotSerial, wantSerial)
	}
}

// TestMatchesDevice_3PARWWNExtraction verifies the WWN is correctly
// extracted from the NAA (strip "naa." prefix).
func TestMatchesDevice_3PARWWNExtraction(t *testing.T) {
	mock := primera3par.NewMockPrimera3ParClient()
	mock.Volumes["vol1"] = populator.LUN{Name: "vol1", SerialNumber: "60002ac0000000000000182d00021f6b"}
	c := primera3par.NewPrimera3ParClonnerForTest(mock)

	got, err := c.MatchesDevice(par3RealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", par3RealNAA)
	}

	wantQuery := fmt.Sprintf("wwn EQ '%s'", "60002ac0000000000000182d00021f6b")
	if mock.LastQuery != wantQuery {
		t.Errorf("query = %q, want %q (WWN not extracted correctly)", mock.LastQuery, wantQuery)
	}
}

// TestMatchesDevice_InfiniboxSerialExtraction verifies the serial is
// correctly extracted from the NAA (strip "naa.6" prefix).
func TestMatchesDevice_InfiniboxSerialExtraction(t *testing.T) {
	wantSerial := "742b0f000000f6f00000000000102a1"
	var gotSerial string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSerial = r.URL.Query().Get("serial")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": []map[string]string{{"serial": wantSerial}},
		})
	}))
	defer server.Close()

	c := infinibox.NewInfiniboxClonnerForTest(server.Listener.Addr().String())
	got, err := c.MatchesDevice(infiniboxRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", infiniboxRealNAA)
	}
	if gotSerial != wantSerial {
		t.Errorf("serial sent to array = %q, want %q (serial not extracted correctly)", gotSerial, wantSerial)
	}
}

// TestMatchesDevice_FlashSystemUIDExtraction verifies the UID is
// correctly extracted from the NAA (strip "naa." prefix).
func TestMatchesDevice_FlashSystemUIDExtraction(t *testing.T) {
	wantUID := "6005076400ce800080000000000004b6"
	var gotBody map[string]string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode([]map[string]string{{"id": "1", "name": "vol1", "vdisk_UID": wantUID}})
	}))
	defer server.Close()

	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to split test server address: %v", err)
	}
	c := flashsystem.NewFlashSystemClonnerForTest(host, port, server.Client())
	got, err := c.MatchesDevice(flashsystemRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", flashsystemRealNAA)
	}
	wantFilter := fmt.Sprintf("vdisk_UID=%s", wantUID)
	if gotBody["filtervalue"] != wantFilter {
		t.Errorf("filtervalue = %q, want %q (UID not extracted correctly)", gotBody["filtervalue"], wantFilter)
	}
}

// TestMatchesDevice_PowerStoreWWNExtraction verifies the WWN is correctly
// extracted from the NAA and sent to the array as-is (lowercased).
func TestMatchesDevice_PowerStoreWWNExtraction(t *testing.T) {
	wantWWN := strings.ToLower(powerstoreRealNAA)
	var gotWWN string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotWWN = r.URL.Query().Get("wwn")
		json.NewEncoder(w).Encode([]map[string]string{{"name": "vol1", "wwn": wantWWN}})
	}))
	defer server.Close()

	c := powerstore.NewPowerstoreClonnerForTest(server.Listener.Addr().String())
	got, err := c.MatchesDevice(powerstoreRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", powerstoreRealNAA)
	}
	wantParam := "eq." + wantWWN
	if gotWWN != wantParam {
		t.Errorf("wwn sent to array = %q, want %q (WWN not extracted correctly)", gotWWN, wantParam)
	}
}

// TestMatchesDevice_PureSerialExtraction verifies the serial is correctly
// extracted from the NAA and uppercased before querying the array.
func TestMatchesDevice_PureSerialExtraction(t *testing.T) {
	wantSerial := "A7B9F7ECC01E40F70001181F"
	var gotFilter string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFilter = r.URL.Query().Get("filter")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]string{{"name": "vol1", "serial": wantSerial}},
		})
	}))
	defer server.Close()

	c := pure.NewFlashArrayClonnerForTest(server.Listener.Addr().String(), server.Client())
	got, err := c.MatchesDevice(pureRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", pureRealNAA)
	}
	wantParam := fmt.Sprintf("serial='%s'", wantSerial)
	if gotFilter != wantParam {
		t.Errorf("filter sent to array = %q, want %q (serial not extracted correctly)", gotFilter, wantParam)
	}
}

// TestMatchesDevice_VantaraLDEVExtraction verifies the LDEV ID is correctly
// decoded from the hex suffix of the NAA.
func TestMatchesDevice_VantaraLDEVExtraction(t *testing.T) {
	mock := &vantara.MockVantaraClientForTest{
		LdevResp:       &vantara.LdevResponse{NaaId: strings.TrimPrefix(vantaraRealNAA, "naa.")},
		LdevStatusCode: http.StatusOK,
	}
	c := vantara.NewVantaraClonnerForTest(mock)

	got, err := c.MatchesDevice(vantaraRealNAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Errorf("MatchesDevice(%q) = false, want true", vantaraRealNAA)
	}
	wantLdevID := "3328"
	if mock.LastLdevID != wantLdevID {
		t.Errorf("LDEV ID sent to array = %q, want %q (LDEV ID not extracted correctly)", mock.LastLdevID, wantLdevID)
	}
}

// TestMatchesDevice_CaseInsensitive verifies all providers handle
// mixed-case NAA device names correctly.
func TestMatchesDevice_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name       string
		provider   storage.ArrayIdentifier
		lower      string
		upper      string
	}{
		{
			"PowerFlex case insensitive",
			powerflex.NewPowerflexClonnerForTest("b4f2d5322f73780f"),
			"eui.b4f2d5322f73780f5a5beec600000002",
			"EUI.B4F2D5322F73780F5A5BEEC600000002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower, err := tt.provider.MatchesDevice(tt.lower)
			if err != nil {
				t.Fatalf("lowercase error: %v", err)
			}
			upper, err := tt.provider.MatchesDevice(tt.upper)
			if err != nil {
				t.Fatalf("uppercase error: %v", err)
			}
			if lower != upper {
				t.Errorf("case mismatch: lower=%v upper=%v", lower, upper)
			}
		})
	}
}
