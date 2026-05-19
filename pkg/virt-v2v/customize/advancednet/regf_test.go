package advancednet

import (
	"encoding/binary"
	"strings"
	"testing"
)

// hiveBuilder creates a minimal synthetic regf hive for testing.
// The hive layout:
//
//	file offset 0x0000: regf header (4096 bytes)
//	file offset 0x1000: hbin (variable)
//
// All cell offsets are relative to 0x1000 (start of first hbin).
type hiveBuilder struct {
	hbin []byte
}

func newHiveBuilder() *hiveBuilder {
	return &hiveBuilder{
		// Start with hbin header (32 bytes)
		hbin: make([]byte, 32),
	}
}

func (b *hiveBuilder) init() {
	copy(b.hbin[0:4], "hbin")
	binary.LittleEndian.PutUint32(b.hbin[4:8], 0)       // offset from start
	binary.LittleEndian.PutUint32(b.hbin[8:12], 0x1000) // size (placeholder, updated at build)
}

// allocCell allocates space for a cell and returns the hive-relative offset.
// The first 4 bytes of the cell are the (negative) size.
func (b *hiveBuilder) allocCell(size int) int32 {
	// Align to 8 bytes
	if size%8 != 0 {
		size += 8 - size%8
	}
	offset := len(b.hbin) // hive-relative offset = absolute - 0x1000
	b.hbin = append(b.hbin, make([]byte, size)...)
	// Write negative cell size (allocated cell)
	negSize := -int32(size)
	binary.LittleEndian.PutUint32(b.hbin[offset:offset+4], uint32(negSize))
	return int32(offset)
}

// writeNK writes an NK cell. Returns the hive-relative offset.
func (b *hiveBuilder) writeNK(name string, subkeysListOff int32, valuesListOff int32, valuesCount uint32, compressed bool) int32 {
	nameBytes := []byte(name)
	// NK body: sig(2) + flags(2) + timestamp(8) + access(4) + parent(4) +
	// subkeyCount(4) + volatileSubkeys(4) + subkeysListOff(4) + volatileListOff(4) +
	// valuesCount(4) + valuesListOff(4) + securityOff(4) + classOff(4) +
	// maxSubkeyNameLen(4) + maxSubkeyClassLen(4) + maxValueNameLen(4) + maxValueDataLen(4) +
	// unused(4) + nameLen(2) + classLen(2) + name(variable)
	// Total fixed: 76 + name
	bodySize := 4 + 76 + len(nameBytes) // cell-size(4) + body(76) + name
	off := b.allocCell(bodySize)
	abs := int(off) + 4 // past cell size

	// Signature "nk"
	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigNK)

	// Flags
	var flags uint16
	if compressed {
		flags |= nkFlagCompressed
	}
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], flags)

	// Body layout (from abs+4, which is past sig+flags):
	bodyStart := abs + 4
	// +0x00: timestamp (8 bytes) - zero
	// +0x08: access bits (4 bytes) - zero
	// +0x0C: parent offset (4 bytes) - zero
	// +0x10: subkeys count (4 bytes)
	// +0x14: volatile subkeys count (4 bytes) - zero
	// +0x18: subkeys list offset (4 bytes)
	// +0x1C: volatile list offset (4 bytes) - -1
	// +0x20: values count (4 bytes)
	// +0x24: values list offset (4 bytes)
	// +0x28: security offset (4 bytes) - -1
	// +0x2C: class offset (4 bytes) - -1
	// +0x30: max subkey name len (4 bytes)
	// +0x34: max subkey class len (4 bytes)
	// +0x38: max value name len (4 bytes)
	// +0x3C: max value data len (4 bytes)
	// +0x40: unused (4 bytes)
	// +0x44: name len (2 bytes)
	// +0x46: class len (2 bytes)
	// +0x48: name (variable)

	// subkeys count — derived from subkeysListOff being non -1
	if subkeysListOff != -1 {
		binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x10:bodyStart+0x14], 1) // placeholder count
	}
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x14:bodyStart+0x18], 0)
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x18:bodyStart+0x1C], uint32(subkeysListOff))
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x1C:bodyStart+0x20], 0xFFFFFFFF) // volatile = -1
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x20:bodyStart+0x24], valuesCount)
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x24:bodyStart+0x28], uint32(valuesListOff))
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x28:bodyStart+0x2C], 0xFFFFFFFF) // security = -1
	binary.LittleEndian.PutUint32(b.hbin[bodyStart+0x2C:bodyStart+0x30], 0xFFFFFFFF) // class = -1
	binary.LittleEndian.PutUint16(b.hbin[bodyStart+0x44:bodyStart+0x46], uint16(len(nameBytes)))
	binary.LittleEndian.PutUint16(b.hbin[bodyStart+0x46:bodyStart+0x48], 0) // class len
	copy(b.hbin[bodyStart+0x48:], nameBytes)

	return off
}

// writeVKDword writes a VK cell with a DWORD value (inline). Returns hive-relative offset.
func (b *hiveBuilder) writeVKDword(name string, value uint32) int32 {
	nameBytes := []byte(name)
	bodySize := 4 + 20 + len(nameBytes) // cell-size + VK header + name
	off := b.allocCell(bodySize)
	abs := int(off) + 4

	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigVK)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nameBytes)))
	// dataSize with high bit set (inline)
	binary.LittleEndian.PutUint32(b.hbin[abs+4:abs+8], 0x80000004)
	// dataOffset = the DWORD value itself (inline)
	binary.LittleEndian.PutUint32(b.hbin[abs+8:abs+12], value)
	// dataType = REG_DWORD
	binary.LittleEndian.PutUint32(b.hbin[abs+12:abs+16], regDWORD)
	// flags: 0x0001 = compressed name
	binary.LittleEndian.PutUint16(b.hbin[abs+16:abs+18], 0x0001)
	copy(b.hbin[abs+20:], nameBytes)

	return off
}

// writeVKMultiSZ writes a VK cell with a REG_MULTI_SZ value. Returns hive-relative offset.
func (b *hiveBuilder) writeVKMultiSZ(name string, values []string) int32 {
	// Build UTF-16LE data: each string null-terminated, followed by extra null
	var u16data []byte
	for _, s := range values {
		for _, r := range s {
			var buf [2]byte
			binary.LittleEndian.PutUint16(buf[:], uint16(r))
			u16data = append(u16data, buf[:]...)
		}
		u16data = append(u16data, 0, 0) // null terminator
	}
	u16data = append(u16data, 0, 0) // double-null terminator

	// Write data cell
	dataCellSize := 4 + len(u16data)
	dataOff := b.allocCell(dataCellSize)
	copy(b.hbin[int(dataOff)+4:], u16data)

	// Write VK cell
	nameBytes := []byte(name)
	bodySize := 4 + 20 + len(nameBytes)
	off := b.allocCell(bodySize)
	abs := int(off) + 4

	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigVK)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nameBytes)))
	binary.LittleEndian.PutUint32(b.hbin[abs+4:abs+8], uint32(len(u16data)))
	binary.LittleEndian.PutUint32(b.hbin[abs+8:abs+12], uint32(dataOff))
	binary.LittleEndian.PutUint32(b.hbin[abs+12:abs+16], regMultiSZ)
	binary.LittleEndian.PutUint16(b.hbin[abs+16:abs+18], 0x0001) // compressed name
	copy(b.hbin[abs+20:], nameBytes)

	return off
}

// writeVKSZ writes a VK cell with a REG_SZ value. Returns hive-relative offset.
func (b *hiveBuilder) writeVKSZ(name, value string) int32 {
	// Build UTF-16LE data with null terminator
	var u16data []byte
	for _, r := range value {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], uint16(r))
		u16data = append(u16data, buf[:]...)
	}
	u16data = append(u16data, 0, 0) // null terminator

	// Write data cell
	dataCellSize := 4 + len(u16data)
	dataOff := b.allocCell(dataCellSize)
	copy(b.hbin[int(dataOff)+4:], u16data)

	// Write VK cell
	nameBytes := []byte(name)
	bodySize := 4 + 20 + len(nameBytes)
	off := b.allocCell(bodySize)
	abs := int(off) + 4

	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigVK)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nameBytes)))
	binary.LittleEndian.PutUint32(b.hbin[abs+4:abs+8], uint32(len(u16data)))
	binary.LittleEndian.PutUint32(b.hbin[abs+8:abs+12], uint32(dataOff))
	binary.LittleEndian.PutUint32(b.hbin[abs+12:abs+16], regSZ)
	binary.LittleEndian.PutUint16(b.hbin[abs+16:abs+18], 0x0001) // compressed name
	copy(b.hbin[abs+20:], nameBytes)

	return off
}

// writeVKBinary writes a VK cell with a REG_BINARY value. Returns hive-relative offset.
func (b *hiveBuilder) writeVKBinary(name string, data []byte) int32 {
	// Write data cell
	dataCellSize := 4 + len(data)
	dataOff := b.allocCell(dataCellSize)
	copy(b.hbin[int(dataOff)+4:], data)

	// Write VK cell
	nameBytes := []byte(name)
	bodySize := 4 + 20 + len(nameBytes)
	off := b.allocCell(bodySize)
	abs := int(off) + 4

	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigVK)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nameBytes)))
	binary.LittleEndian.PutUint32(b.hbin[abs+4:abs+8], uint32(len(data)))
	binary.LittleEndian.PutUint32(b.hbin[abs+8:abs+12], uint32(dataOff))
	binary.LittleEndian.PutUint32(b.hbin[abs+12:abs+16], regBinary)
	binary.LittleEndian.PutUint16(b.hbin[abs+16:abs+18], 0x0001) // compressed name
	copy(b.hbin[abs+20:], nameBytes)

	return off
}

// writeValuesList writes a values list cell containing the given VK offsets.
func (b *hiveBuilder) writeValuesList(vkOffsets []int32) int32 {
	size := 4 + len(vkOffsets)*4
	off := b.allocCell(size)
	abs := int(off) + 4
	for i, vkOff := range vkOffsets {
		binary.LittleEndian.PutUint32(b.hbin[abs+i*4:abs+i*4+4], uint32(vkOff))
	}
	return off
}

// writeLH writes an LH (hash-based) subkey index. Returns hive-relative offset.
func (b *hiveBuilder) writeLH(nkOffsets []int32) int32 {
	// Each element: 4 bytes NK offset + 4 bytes hash
	size := 4 + 4 + len(nkOffsets)*8 // cell-size + sig(2)+count(2) + elements
	off := b.allocCell(size)
	abs := int(off) + 4
	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigLH)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nkOffsets)))
	for i, nkOff := range nkOffsets {
		elemOff := abs + 4 + i*8
		binary.LittleEndian.PutUint32(b.hbin[elemOff:elemOff+4], uint32(nkOff))
		// hash = 0 (we don't check it)
	}
	return off
}

// build produces the complete hive byte slice (header + hbin).
func (b *hiveBuilder) build(rootCellOff int32) []byte {
	// Update hbin size
	hbinSize := len(b.hbin)
	if hbinSize%4096 != 0 {
		hbinSize += 4096 - hbinSize%4096
	}
	if hbinSize < 4096 {
		hbinSize = 4096
	}
	padded := make([]byte, hbinSize)
	copy(padded, b.hbin)
	binary.LittleEndian.PutUint32(padded[8:12], uint32(hbinSize))

	header := make([]byte, 4096)
	copy(header[0:4], "regf")
	binary.LittleEndian.PutUint32(header[36:40], uint32(rootCellOff))

	result := make([]byte, 0, len(header)+len(padded))
	result = append(result, header...)
	result = append(result, padded...)
	return result
}

// buildTestHive creates a synthetic SYSTEM hive with:
//   - Select\Current = 1 (ControlSet001)
//   - ControlSet001\Services\Tcpip\Parameters\Interfaces\{TEST-GUID-1}
//   - InterfaceMetric = 25
//   - RegistrationEnabled = 0
//   - ControlSet001\Services\NetBT\Parameters\Interfaces\Tcpip_{TEST-GUID-1}
//   - NetbiosOptions = 2
//   - ControlSet001\Services\LanmanServer
//   - Start = 4  (disabled)
//   - ControlSet001\Services\LanmanServer\Linkage
//   - Bind = ["\Device\NetBT_Tcpip_{TEST-GUID-2}"]  (GUID-1 not bound = F&PS disabled)
func buildTestHive() []byte {
	b := newHiveBuilder()
	b.init()

	// === Values ===

	// Select\Current = 1
	vkSelectCurrent := b.writeVKDword("Current", 1)

	// InterfaceMetric = 25
	vkInterfaceMetric := b.writeVKDword("InterfaceMetric", 25)
	// RegistrationEnabled = 0
	vkRegEnabled := b.writeVKDword("RegistrationEnabled", 0)

	// NetbiosOptions = 2
	vkNetbios := b.writeVKDword("NetbiosOptions", 2)

	// LanmanServer Start = 4
	vkLanmanStart := b.writeVKDword("Start", 4)

	// LanmanServer\Linkage Bind (only GUID-2 bound, GUID-1 missing = disabled)
	vkBind := b.writeVKMultiSZ("Bind", []string{`\Device\NetBT_Tcpip_{TEST-GUID-2}`})

	// === Value lists ===
	vlSelect := b.writeValuesList([]int32{vkSelectCurrent})
	vlInterface := b.writeValuesList([]int32{vkInterfaceMetric, vkRegEnabled})
	vlNetbt := b.writeValuesList([]int32{vkNetbios})
	vlLanman := b.writeValuesList([]int32{vkLanmanStart})
	vlLinkage := b.writeValuesList([]int32{vkBind})

	// === NK cells (bottom-up) ===

	// Select key
	nkSelect := b.writeNK("Select", -1, vlSelect, 1, true)

	// {TEST-GUID-1} under Tcpip\Parameters\Interfaces
	nkGUID1 := b.writeNK("{TEST-GUID-1}", -1, vlInterface, 2, true)

	// {TEST-GUID-2} under Tcpip\Parameters\Interfaces (no custom values)
	nkGUID2 := b.writeNK("{TEST-GUID-2}", -1, -1, 0, true)

	// Interfaces (parent of GUID keys)
	lhInterfaces := b.writeLH([]int32{nkGUID1, nkGUID2})
	nkInterfaces := b.writeNK("Interfaces", lhInterfaces, -1, 0, true)

	// Parameters
	lhParameters := b.writeLH([]int32{nkInterfaces})
	nkParameters := b.writeNK("Parameters", lhParameters, -1, 0, true)

	// Tcpip
	lhTcpip := b.writeLH([]int32{nkParameters})
	nkTcpip := b.writeNK("Tcpip", lhTcpip, -1, 0, true)

	// Tcpip_{TEST-GUID-1} under NetBT\Parameters\Interfaces
	nkNetbtGUID1 := b.writeNK("Tcpip_{TEST-GUID-1}", -1, vlNetbt, 1, true)

	// NetBT Interfaces
	lhNetbtInterfaces := b.writeLH([]int32{nkNetbtGUID1})
	nkNetbtInterfaces := b.writeNK("Interfaces", lhNetbtInterfaces, -1, 0, true)

	// NetBT Parameters
	lhNetbtParams := b.writeLH([]int32{nkNetbtInterfaces})
	nkNetbtParams := b.writeNK("Parameters", lhNetbtParams, -1, 0, true)

	// NetBT
	lhNetbt := b.writeLH([]int32{nkNetbtParams})
	nkNetbt := b.writeNK("NetBT", lhNetbt, -1, 0, true)

	// Linkage under LanmanServer
	nkLinkage := b.writeNK("Linkage", -1, vlLinkage, 1, true)

	// LanmanServer
	lhLanmanSubs := b.writeLH([]int32{nkLinkage})
	nkLanman := b.writeNK("LanmanServer", lhLanmanSubs, vlLanman, 1, true)

	// Services (parent of Tcpip, NetBT, LanmanServer)
	lhServices := b.writeLH([]int32{nkTcpip, nkNetbt, nkLanman})
	nkServices := b.writeNK("Services", lhServices, -1, 0, true)

	// ControlSet001
	lhCS001 := b.writeLH([]int32{nkServices})
	nkCS001 := b.writeNK("ControlSet001", lhCS001, -1, 0, true)

	// Root key (CMI-CreateHive) with children: Select, ControlSet001
	lhRoot := b.writeLH([]int32{nkSelect, nkCS001})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)

	return b.build(nkRoot)
}

func TestParseHive_InvalidSignature(t *testing.T) {
	data := make([]byte, 8192)
	copy(data[0:4], "BAAD")
	_, err := ParseHive(data)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestParseHive_TooShort(t *testing.T) {
	data := make([]byte, 100)
	copy(data[0:4], "regf")
	_, err := ParseHive(data)
	if err == nil {
		t.Fatal("expected error for data too short")
	}
}

func TestParseHive_ValidHeader(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	root, err := hive.RootKey()
	if err != nil {
		t.Fatalf("RootKey: %v", err)
	}
	if root == nil {
		t.Fatal("root key is nil")
	}
}

func TestResolveCurrentControlSet(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	cs, err := hive.ResolveCurrentControlSet()
	if err != nil {
		t.Fatalf("ResolveCurrentControlSet: %v", err)
	}
	if cs != "ControlSet001" {
		t.Fatalf("expected ControlSet001, got %s", cs)
	}
}

func TestOpenKey(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}

	tests := []struct {
		name   string
		path   string
		exists bool
	}{
		{"Select key", "Select", true},
		{"ControlSet001 key", "ControlSet001", true},
		{"Services key", "ControlSet001\\Services", true},
		{"Tcpip Interfaces", "ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces", true},
		{"GUID-1 interface", "ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces\\{TEST-GUID-1}", true},
		{"NetBT GUID-1", "ControlSet001\\Services\\NetBT\\Parameters\\Interfaces\\Tcpip_{TEST-GUID-1}", true},
		{"LanmanServer key", "ControlSet001\\Services\\LanmanServer", true},
		{"LanmanServer Linkage", "ControlSet001\\Services\\LanmanServer\\Linkage", true},
		{"NonExistent key", "NonExistent", false},
		{"NonExistent service", "ControlSet001\\Services\\NonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nk, err := hive.OpenKey(tt.path)
			if err != nil {
				t.Fatalf("OpenKey(%q): %v", tt.path, err)
			}
			if tt.exists && nk == nil {
				t.Errorf("OpenKey(%q): expected key to exist", tt.path)
			}
			if !tt.exists && nk != nil {
				t.Errorf("OpenKey(%q): expected key to not exist", tt.path)
			}
		})
	}
}

func TestReadDWORD(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}

	nk, err := hive.OpenKey("ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces\\{TEST-GUID-1}")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v (nk=%v)", err, nk)
	}

	metric, found, err := hive.ReadDWORD(nk, "InterfaceMetric")
	if err != nil {
		t.Fatalf("ReadDWORD InterfaceMetric: %v", err)
	}
	if !found {
		t.Fatal("InterfaceMetric not found")
	}
	if metric != 25 {
		t.Fatalf("InterfaceMetric: expected 25, got %d", metric)
	}

	regEnabled, found, err := hive.ReadDWORD(nk, "RegistrationEnabled")
	if err != nil {
		t.Fatalf("ReadDWORD RegistrationEnabled: %v", err)
	}
	if !found {
		t.Fatal("RegistrationEnabled not found")
	}
	if regEnabled != 0 {
		t.Fatalf("RegistrationEnabled: expected 0, got %d", regEnabled)
	}

	_, found, err = hive.ReadDWORD(nk, "NonExistent")
	if err != nil {
		t.Fatalf("ReadDWORD NonExistent: %v", err)
	}
	if found {
		t.Fatal("NonExistent should not be found")
	}
}

func TestReadMultiSZ(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}

	nk, err := hive.OpenKey("ControlSet001\\Services\\LanmanServer\\Linkage")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v (nk=%v)", err, nk)
	}

	bind, found, err := hive.ReadMultiSZ(nk, "Bind")
	if err != nil {
		t.Fatalf("ReadMultiSZ Bind: %v", err)
	}
	if !found {
		t.Fatal("Bind not found")
	}
	if len(bind) != 1 {
		t.Fatalf("Bind: expected 1 entry, got %d", len(bind))
	}
	if bind[0] != `\Device\NetBT_Tcpip_{TEST-GUID-2}` {
		t.Fatalf("Bind[0]: expected \\Device\\NetBT_Tcpip_{TEST-GUID-2}, got %q", bind[0])
	}
}

func TestEnumerateSubkeys(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}

	nk, err := hive.OpenKey("ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}

	children, err := hive.EnumerateSubkeys(nk)
	if err != nil {
		t.Fatalf("EnumerateSubkeys: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 interface subkeys, got %d", len(children))
	}
	names := map[string]bool{}
	for _, c := range children {
		names[c.name] = true
	}
	if !names["{TEST-GUID-1}"] || !names["{TEST-GUID-2}"] {
		t.Fatalf("unexpected subkey names: %v", names)
	}
}

func TestParseAdvancedNetworkSettings(t *testing.T) {
	data := buildTestHive()
	settings, err := ParseAdvancedNetworkSettings(data)
	if err != nil {
		t.Fatalf("ParseAdvancedNetworkSettings: %v", err)
	}

	if settings.LanmanServerStart != 4 {
		t.Errorf("LanmanServerStart: expected 4, got %d", settings.LanmanServerStart)
	}

	// buildTestHive has no NetworkSetup2 keys and no NetworkAddress values,
	// so adapters can't be resolved to MACs and are skipped.
	if len(settings.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces without MAC source, got %d", len(settings.Interfaces))
	}
	if len(settings.FilePrinterSharingDisabled) != 0 {
		t.Errorf("expected 0 F&PS disabled adapters without MAC source, got %d", len(settings.FilePrinterSharingDisabled))
	}
}

func TestHasNonDefaultSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings AdvancedNetSettings
		expected bool
	}{
		{
			name:     "empty settings",
			settings: AdvancedNetSettings{},
			expected: false,
		},
		{
			name: "LanmanServer disabled",
			settings: AdvancedNetSettings{
				LanmanServerStart: 4,
			},
			expected: true,
		},
		{
			name: "LanmanServer auto (default)",
			settings: AdvancedNetSettings{
				LanmanServerStart: 2,
			},
			expected: false,
		},
		{
			name: "F&PS disabled adapter",
			settings: AdvancedNetSettings{
				FilePrinterSharingDisabled: []AdapterRef{{GUID: "{GUID}"}},
			},
			expected: true,
		},
		{
			name: "custom metric",
			settings: AdvancedNetSettings{
				Interfaces: []InterfaceSettings{
					{InterfaceMetric: 10},
				},
			},
			expected: true,
		},
		{
			name: "DNS registration disabled (explicit)",
			settings: AdvancedNetSettings{
				Interfaces: []InterfaceSettings{
					{RegistrationEnabled: 3},
				},
			},
			expected: true,
		},
		{
			name: "DNS registration disabled (value 0)",
			settings: AdvancedNetSettings{
				Interfaces: []InterfaceSettings{
					{RegistrationEnabled: 0},
				},
			},
			expected: true,
		},
		{
			name: "NetBIOS enabled",
			settings: AdvancedNetSettings{
				Interfaces: []InterfaceSettings{
					{NetbiosOptions: 1},
				},
			},
			expected: true,
		},
		{
			name: "auto metric (default)",
			settings: AdvancedNetSettings{
				Interfaces: []InterfaceSettings{
					{InterfaceMetricAuto: true, RegistrationEnabled: 1},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.settings.HasNonDefaultSettings()
			if got != tt.expected {
				t.Errorf("HasNonDefaultSettings() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDecodeUTF16LE(t *testing.T) {
	// "ABC" in UTF-16LE
	input := []byte{0x41, 0x00, 0x42, 0x00, 0x43, 0x00}
	result := decodeUTF16LE(input)
	if result != "ABC" {
		t.Errorf("expected ABC, got %q", result)
	}
}

func TestWriteAndReadSettingsFile(t *testing.T) {
	dir := t.TempDir()
	settings := &AdvancedNetSettings{
		Interfaces: []InterfaceSettings{
			{
				MAC:                 "{TEST-GUID}",
				InterfaceMetric:     25,
				RegistrationEnabled: 0,
				NetbiosOptions:      2,
			},
		},
		LanmanServerStart:          4,
		FilePrinterSharingDisabled: []AdapterRef{{GUID: "{TEST-GUID}", MAC: "00:11:22:33:44:55"}},
	}

	err := WriteSettingsFile(settings, dir)
	if err != nil {
		t.Fatalf("WriteSettingsFile: %v", err)
	}

	read, err := ReadSettingsFile(dir)
	if err != nil {
		t.Fatalf("ReadSettingsFile: %v", err)
	}
	if read == nil {
		t.Fatal("ReadSettingsFile returned nil")
	}
	if len(read.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(read.Interfaces))
	}
	if read.Interfaces[0].InterfaceMetric != 25 {
		t.Errorf("InterfaceMetric: expected 25, got %d", read.Interfaces[0].InterfaceMetric)
	}
	if read.LanmanServerStart != 4 {
		t.Errorf("LanmanServerStart: expected 4, got %d", read.LanmanServerStart)
	}
}

func TestReadSettingsFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	read, err := ReadSettingsFile(dir)
	if err != nil {
		t.Fatalf("ReadSettingsFile: %v", err)
	}
	if read != nil {
		t.Fatal("expected nil for non-existent file")
	}
}

// buildTestHiveWithNetworkSetup2 creates a hive with NetworkSetup2\Interfaces
// containing CurrentAddress (REG_BINARY) for direct GUID-to-MAC resolution.
func buildTestHiveWithNetworkSetup2() []byte {
	b := newHiveBuilder()
	b.init()

	// === Values ===
	vkSelectCurrent := b.writeVKDword("Current", 1)
	vkInterfaceMetric := b.writeVKDword("InterfaceMetric", 25)
	vkRegEnabled := b.writeVKDword("RegistrationEnabled", 0)
	vkNetbios := b.writeVKDword("NetbiosOptions", 2)
	vkLanmanStart := b.writeVKDword("Start", 4)
	vkBind := b.writeVKMultiSZ("Bind", []string{`\Device\NetBT_Tcpip_{TEST-GUID-2}`})

	// NetworkSetup2 CurrentAddress values (6-byte raw MAC)
	vkMAC1 := b.writeVKBinary("CurrentAddress", []byte{0x00, 0x50, 0x56, 0xBE, 0x56, 0xA1})
	vkMAC2 := b.writeVKBinary("CurrentAddress", []byte{0x00, 0x50, 0x56, 0xBE, 0x56, 0xA2})

	// === Value lists ===
	vlSelect := b.writeValuesList([]int32{vkSelectCurrent})
	vlInterface := b.writeValuesList([]int32{vkInterfaceMetric, vkRegEnabled})
	vlNetbt := b.writeValuesList([]int32{vkNetbios})
	vlLanman := b.writeValuesList([]int32{vkLanmanStart})
	vlLinkage := b.writeValuesList([]int32{vkBind})
	vlKernel1 := b.writeValuesList([]int32{vkMAC1})
	vlKernel2 := b.writeValuesList([]int32{vkMAC2})

	// === NK cells (bottom-up) ===
	nkSelect := b.writeNK("Select", -1, vlSelect, 1, true)

	// Tcpip\Parameters\Interfaces
	nkGUID1 := b.writeNK("{TEST-GUID-1}", -1, vlInterface, 2, true)
	nkGUID2 := b.writeNK("{TEST-GUID-2}", -1, -1, 0, true)
	lhInterfaces := b.writeLH([]int32{nkGUID1, nkGUID2})
	nkInterfaces := b.writeNK("Interfaces", lhInterfaces, -1, 0, true)
	lhParameters := b.writeLH([]int32{nkInterfaces})
	nkParameters := b.writeNK("Parameters", lhParameters, -1, 0, true)
	lhTcpip := b.writeLH([]int32{nkParameters})
	nkTcpip := b.writeNK("Tcpip", lhTcpip, -1, 0, true)

	// NetBT
	nkNetbtGUID1 := b.writeNK("Tcpip_{TEST-GUID-1}", -1, vlNetbt, 1, true)
	lhNetbtInterfaces := b.writeLH([]int32{nkNetbtGUID1})
	nkNetbtInterfaces := b.writeNK("Interfaces", lhNetbtInterfaces, -1, 0, true)
	lhNetbtParams := b.writeLH([]int32{nkNetbtInterfaces})
	nkNetbtParams := b.writeNK("Parameters", lhNetbtParams, -1, 0, true)
	lhNetbt := b.writeLH([]int32{nkNetbtParams})
	nkNetbt := b.writeNK("NetBT", lhNetbt, -1, 0, true)

	// LanmanServer
	nkLinkage := b.writeNK("Linkage", -1, vlLinkage, 1, true)
	lhLanmanSubs := b.writeLH([]int32{nkLinkage})
	nkLanman := b.writeNK("LanmanServer", lhLanmanSubs, vlLanman, 1, true)

	// NetworkSetup2\Interfaces\{GUID}\Kernel
	nkKernel1 := b.writeNK("Kernel", -1, vlKernel1, 1, true)
	lhNS2GUID1 := b.writeLH([]int32{nkKernel1})
	nkNS2GUID1 := b.writeNK("{TEST-GUID-1}", lhNS2GUID1, -1, 0, true)

	nkKernel2 := b.writeNK("Kernel", -1, vlKernel2, 1, true)
	lhNS2GUID2 := b.writeLH([]int32{nkKernel2})
	nkNS2GUID2 := b.writeNK("{TEST-GUID-2}", lhNS2GUID2, -1, 0, true)

	lhNS2Interfaces := b.writeLH([]int32{nkNS2GUID1, nkNS2GUID2})
	nkNS2Interfaces := b.writeNK("Interfaces", lhNS2Interfaces, -1, 0, true)
	lhNetworkSetup2 := b.writeLH([]int32{nkNS2Interfaces})
	nkNetworkSetup2 := b.writeNK("NetworkSetup2", lhNetworkSetup2, -1, 0, true)

	// Control (parent of NetworkSetup2)
	lhControl := b.writeLH([]int32{nkNetworkSetup2})
	nkControl := b.writeNK("Control", lhControl, -1, 0, true)

	// Services
	lhServices := b.writeLH([]int32{nkTcpip, nkNetbt, nkLanman})
	nkServices := b.writeNK("Services", lhServices, -1, 0, true)

	// ControlSet001
	lhCS001 := b.writeLH([]int32{nkServices, nkControl})
	nkCS001 := b.writeNK("ControlSet001", lhCS001, -1, 0, true)

	// Root
	lhRoot := b.writeLH([]int32{nkSelect, nkCS001})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)

	return b.build(nkRoot)
}

func TestParseAdvancedNetworkSettings_NetworkSetup2(t *testing.T) {
	data := buildTestHiveWithNetworkSetup2()
	settings, err := ParseAdvancedNetworkSettings(data)
	if err != nil {
		t.Fatalf("ParseAdvancedNetworkSettings: %v", err)
	}

	// {TEST-GUID-1} has InterfaceMetric=25, RegistrationEnabled=0 (non-default)
	if len(settings.Interfaces) != 1 {
		t.Fatalf("expected 1 interface with non-default settings, got %d", len(settings.Interfaces))
	}
	iface := settings.Interfaces[0]
	if iface.MAC != "00:50:56:BE:56:A1" {
		t.Errorf("MAC: expected 00:50:56:BE:56:A1, got %s", iface.MAC)
	}
	if iface.InterfaceMetric != 25 {
		t.Errorf("InterfaceMetric: expected 25, got %d", iface.InterfaceMetric)
	}
	if iface.RegistrationEnabled != 0 {
		t.Errorf("RegistrationEnabled: expected 0, got %d", iface.RegistrationEnabled)
	}

	// F&PS: {TEST-GUID-1} is unbound (only GUID-2 in Bind list)
	if len(settings.FilePrinterSharingDisabled) != 1 {
		t.Fatalf("expected 1 F&PS disabled adapter, got %d", len(settings.FilePrinterSharingDisabled))
	}
	fps := settings.FilePrinterSharingDisabled[0]
	if fps.GUID != "{TEST-GUID-1}" {
		t.Errorf("F&PS GUID: expected {TEST-GUID-1}, got %s", fps.GUID)
	}
	if fps.MAC != "00:50:56:BE:56:A1" {
		t.Errorf("F&PS MAC: expected 00:50:56:BE:56:A1, got %s", fps.MAC)
	}

	if settings.LanmanServerStart != 4 {
		t.Errorf("LanmanServerStart: expected 4, got %d", settings.LanmanServerStart)
	}
}

func TestParseAdvancedNetworkSettings_NoMACSource(t *testing.T) {
	data := buildTestHive()

	// buildTestHive has no NetworkSetup2 and no NetworkAddress values,
	// so no GUID-to-MAC mapping is possible.
	settings, err := ParseAdvancedNetworkSettings(data)
	if err != nil {
		t.Fatalf("ParseAdvancedNetworkSettings: %v", err)
	}
	if len(settings.Interfaces) != 0 {
		t.Errorf("expected 0 interfaces without MAC source, got %d", len(settings.Interfaces))
	}
}

func TestReadSZ(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	vkHello := b.writeVKSZ("Greeting", "Hello")
	vl := b.writeValuesList([]int32{vkHello})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v (nk=%v)", err, nk)
	}

	val, found, err := hive.ReadSZ(nk, "Greeting")
	if err != nil {
		t.Fatalf("ReadSZ: %v", err)
	}
	if !found {
		t.Fatal("Greeting not found")
	}
	if val != "Hello" {
		t.Fatalf("expected %q, got %q", "Hello", val)
	}

	_, found, err = hive.ReadSZ(nk, "Missing")
	if err != nil {
		t.Fatalf("ReadSZ Missing: %v", err)
	}
	if found {
		t.Fatal("Missing should not be found")
	}
}

func TestReadBinary(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	mac := []byte{0x00, 0x50, 0x56, 0xBE, 0x56, 0xA1}
	vkMAC := b.writeVKBinary("CurrentAddress", mac)
	vl := b.writeValuesList([]int32{vkMAC})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v (nk=%v)", err, nk)
	}

	val, found, err := hive.ReadBinary(nk, "CurrentAddress")
	if err != nil {
		t.Fatalf("ReadBinary: %v", err)
	}
	if !found {
		t.Fatal("CurrentAddress not found")
	}
	if len(val) != 6 {
		t.Fatalf("expected 6 bytes, got %d", len(val))
	}
	for i, b := range mac {
		if val[i] != b {
			t.Fatalf("byte %d: expected 0x%02X, got 0x%02X", i, b, val[i])
		}
	}

	_, found, err = hive.ReadBinary(nk, "Missing")
	if err != nil {
		t.Fatalf("ReadBinary Missing: %v", err)
	}
	if found {
		t.Fatal("Missing should not be found")
	}
}

func TestReadDWORD_TypeMismatch(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	vkSZ := b.writeVKSZ("NotADword", "hello")
	vl := b.writeValuesList([]int32{vkSZ})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}

	_, _, err = hive.ReadDWORD(nk, "NotADword")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "expected DWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBinary_TypeMismatch(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	vkDW := b.writeVKDword("NotBinary", 42)
	vl := b.writeValuesList([]int32{vkDW})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}

	_, _, err = hive.ReadBinary(nk, "NotBinary")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "expected BINARY") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadMultiSZ_TypeMismatch(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	vkDW := b.writeVKDword("NotMultiSZ", 99)
	vl := b.writeValuesList([]int32{vkDW})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}

	_, _, err = hive.ReadMultiSZ(nk, "NotMultiSZ")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "expected MULTI_SZ") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadBinary_InlineData(t *testing.T) {
	b := newHiveBuilder()
	b.init()

	// Manually write a VK cell with inline binary data (2 bytes, bit 31 set)
	nameBytes := []byte("TinyMAC")
	bodySize := 4 + 20 + len(nameBytes)
	off := b.allocCell(bodySize)
	abs := int(off) + 4

	binary.LittleEndian.PutUint16(b.hbin[abs:abs+2], sigVK)
	binary.LittleEndian.PutUint16(b.hbin[abs+2:abs+4], uint16(len(nameBytes)))
	// dataSize = 2 | 0x80000000 (inline, 2 bytes)
	binary.LittleEndian.PutUint32(b.hbin[abs+4:abs+8], 0x80000002)
	// dataOffset holds the inline data: [0xAB, 0xCD, 0x00, 0x00]
	binary.LittleEndian.PutUint32(b.hbin[abs+8:abs+12], 0x0000CDAB)
	binary.LittleEndian.PutUint32(b.hbin[abs+12:abs+16], regBinary)
	binary.LittleEndian.PutUint16(b.hbin[abs+16:abs+18], 0x0001)
	copy(b.hbin[abs+20:], nameBytes)

	vl := b.writeValuesList([]int32{off})
	nkTestKey := b.writeNK("TestKey", -1, vl, 1, true)
	lhRoot := b.writeLH([]int32{nkTestKey})
	nkRoot := b.writeNK("CMI-CreateHive{2A7FB991-7BBE-4F9D-B91E-7CB51D4737F5}", lhRoot, -1, 0, true)
	data := b.build(nkRoot)

	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("TestKey")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}

	val, found, err := hive.ReadBinary(nk, "TinyMAC")
	if err != nil {
		t.Fatalf("ReadBinary inline: %v", err)
	}
	if !found {
		t.Fatal("TinyMAC not found")
	}
	if len(val) != 2 || val[0] != 0xAB || val[1] != 0xCD {
		t.Fatalf("expected [0xAB, 0xCD], got %v", val)
	}
}

func TestOpenKey_EmptyPath(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("")
	if err != nil {
		t.Fatalf("OpenKey empty: %v", err)
	}
	if nk == nil {
		t.Fatal("expected root key for empty path")
	}
}

func TestOpenKey_CaseInsensitive(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("CONTROLSET001\\SERVICES\\TCPIP")
	if err != nil {
		t.Fatalf("OpenKey case-insensitive: %v", err)
	}
	if nk == nil {
		t.Fatal("expected key to exist (case-insensitive)")
	}
}

func TestEnumerateSubkeys_NoChildren(t *testing.T) {
	data := buildTestHive()
	hive, err := ParseHive(data)
	if err != nil {
		t.Fatalf("ParseHive: %v", err)
	}
	nk, err := hive.OpenKey("ControlSet001\\Services\\Tcpip\\Parameters\\Interfaces\\{TEST-GUID-2}")
	if err != nil || nk == nil {
		t.Fatalf("OpenKey: %v", err)
	}
	children, err := hive.EnumerateSubkeys(nk)
	if err != nil {
		t.Fatalf("EnumerateSubkeys: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("expected 0 children, got %d", len(children))
	}
}

func TestParseAdvancedNetworkSettings_CorruptHive(t *testing.T) {
	// Take a valid hive and corrupt an internal offset so that parsing
	// reaches a point where a read helper slices out of bounds.
	data := buildTestHive()

	// Corrupt the root cell's subkeys-list offset to point far beyond the data,
	// bypassing the NK parse (which succeeds) but panicking when following
	// the bogus subkeys-list pointer.
	rootOff := int(binary.LittleEndian.Uint32(data[36:40]))
	abs := rootOff + 0x1000 + 4 // past cell-size
	bodyStart := abs + 4        // past sig + flags
	// Overwrite subkeysListOff with a value that's within hbin range check
	// but will cause OOB in the LH/LF parsing due to element count mismatch.
	binary.LittleEndian.PutUint32(data[bodyStart+0x18:bodyStart+0x1C], uint32(len(data)-0x1000-8))

	_, err := ParseAdvancedNetworkSettings(data)
	if err == nil {
		t.Fatal("expected error for corrupt hive, got nil")
	}
	if !strings.Contains(err.Error(), "corrupt") && !strings.Contains(err.Error(), "regf") {
		t.Errorf("expected regf-related error, got: %v", err)
	}
}
