package advancednet

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// Registry value types
const (
	regNone     = 0
	regSZ       = 1
	regExpandSZ = 2
	regBinary   = 3
	regDWORD    = 4
	regMultiSZ  = 7
	regQWORD    = 11
)

// Cell signatures
const (
	sigRegf = "regf"
	sigHbin = "hbin"
	sigNK   = 0x6B6E // "nk" little-endian
	sigVK   = 0x6B76 // "vk" little-endian
	sigLF   = 0x666C // "lf" little-endian
	sigLH   = 0x686C // "lh" little-endian
	sigLI   = 0x696C // "li" little-endian
	sigRI   = 0x6972 // "ri" little-endian
)

// nkFlagCompressed indicates the key name is stored as ASCII (compressed).
const nkFlagCompressed = 0x0020

// Common cell layout sizes.
const (
	cellSizeLen = 4 // int32 cell-size prefix on every cell
	sigLen      = 2 // uint16 signature ("nk", "vk", "lf", etc.)
	flagsLen    = 2 // uint16 flags field
)

// NK cell body offsets (relative to body start, after sig + flags).
const (
	nkOffSubkeysList = 0x18
	nkOffValuesCount = 0x20
	nkOffValuesList  = 0x24
	nkOffNameLen     = 0x44
	nkOffNameData    = 0x48
	nkMinSize        = sigLen + flagsLen + nkOffNameData // from sig to start of variable name (checked after skipping cell-size)
)

// VK cell offsets (relative to start of VK record, after cell-size prefix).
const (
	vkOffNameLen    = 2
	vkOffDataSize   = 4
	vkOffDataOffset = 8
	vkOffDataType   = 12
	vkOffFlags      = 16
	vkOffNameData   = 20
	vkMinSize       = vkOffNameData // from sig to start of variable name (checked after skipping cell-size)
)

// Subkey list header: cell-size(4) + sig(2) + count(2).
// Checked before skipping cell-size, so includes cellSizeLen.
const subkeyListHeaderSize = cellSizeLen + sigLen + flagsLen

// Hive represents a parsed Windows NT registry hive (regf format).
type Hive struct {
	data     []byte
	rootCell int32
}

// nkCell is a parsed NK (key node) record.
type nkCell struct {
	subkeysListOffset int32
	valuesListOffset  int32
	valuesCount       uint32
	name              string
}

// vkCell is a parsed VK (value) record.
type vkCell struct {
	name     string
	dataType uint32
	dataSize uint32
	// dataOffset stores the raw offset field. When dataSize <= 4 the data is
	// stored inline (the high bit of dataSize is set).
	dataOffset uint32
}

// ParseHive validates a regf header and returns a Hive handle.
func ParseHive(data []byte) (*Hive, error) {
	if len(data) < 4096 {
		return nil, fmt.Errorf("regf: data too short (%d bytes)", len(data))
	}
	if string(data[0:4]) != sigRegf {
		return nil, fmt.Errorf("regf: invalid signature %q", string(data[0:4]))
	}
	rootOffset := int32(binary.LittleEndian.Uint32(data[36:40]))
	return &Hive{
		data:     data,
		rootCell: rootOffset,
	}, nil
}

// cellOffset translates a hive-relative offset (from the root of the first
// hbin) to an absolute byte offset in the data slice. All registry offsets
// are relative to the start of the hbin data at file offset 0x1000.
func cellOffset(off int32) int {
	return int(off) + 0x1000
}

// readInt32 reads a signed 32-bit LE value.
func (h *Hive) readInt32(off int) int32 {
	return int32(binary.LittleEndian.Uint32(h.data[off : off+4]))
}

// readUint32 reads an unsigned 32-bit LE value.
func (h *Hive) readUint32(off int) uint32 {
	return binary.LittleEndian.Uint32(h.data[off : off+4])
}

// readUint16 reads an unsigned 16-bit LE value.
func (h *Hive) readUint16(off int) uint16 {
	return binary.LittleEndian.Uint16(h.data[off : off+2])
}

// parseNK parses an NK cell at the given hive-relative offset.
func (h *Hive) parseNK(off int32) (*nkCell, error) {
	abs := cellOffset(off)
	if abs < 0 || abs+cellSizeLen > len(h.data) {
		return nil, fmt.Errorf("regf: NK offset 0x%x out of range", off)
	}
	abs += cellSizeLen
	if abs+nkMinSize > len(h.data) {
		return nil, fmt.Errorf("regf: NK record too short at offset 0x%x", off)
	}
	sig := h.readUint16(abs)
	if sig != sigNK {
		return nil, fmt.Errorf("regf: expected NK signature, got 0x%04x at 0x%x", sig, off)
	}
	flags := h.readUint16(abs + sigLen)

	bodyStart := abs + sigLen + flagsLen
	subkeysListOff := h.readInt32(bodyStart + nkOffSubkeysList)
	valuesCount := h.readUint32(bodyStart + nkOffValuesCount)
	valuesListOff := h.readInt32(bodyStart + nkOffValuesList)
	nameLen := h.readUint16(bodyStart + nkOffNameLen)

	nameStart := bodyStart + nkOffNameData
	if nameStart+int(nameLen) > len(h.data) {
		return nil, fmt.Errorf("regf: NK name overflows at 0x%x", off)
	}
	var name string
	if flags&nkFlagCompressed != 0 {
		name = string(h.data[nameStart : nameStart+int(nameLen)])
	} else {
		name = decodeUTF16LE(h.data[nameStart : nameStart+int(nameLen)])
	}

	return &nkCell{
		subkeysListOffset: subkeysListOff,
		valuesListOffset:  valuesListOff,
		valuesCount:       valuesCount,
		name:              name,
	}, nil
}

// parseVK parses a VK cell at the given hive-relative offset.
func (h *Hive) parseVK(off int32) (*vkCell, error) {
	abs := cellOffset(off)
	if abs < 0 || abs+cellSizeLen > len(h.data) {
		return nil, fmt.Errorf("regf: VK offset 0x%x out of range", off)
	}
	abs += cellSizeLen
	if abs+vkMinSize > len(h.data) {
		return nil, fmt.Errorf("regf: VK record too short at offset 0x%x", off)
	}
	sig := h.readUint16(abs)
	if sig != sigVK {
		return nil, fmt.Errorf("regf: expected VK signature, got 0x%04x at 0x%x", sig, off)
	}
	nameLen := h.readUint16(abs + vkOffNameLen)
	dataSize := h.readUint32(abs + vkOffDataSize)
	dataOffset := h.readUint32(abs + vkOffDataOffset)
	dataType := h.readUint32(abs + vkOffDataType)
	flags := h.readUint16(abs + vkOffFlags)

	nameStart := abs + vkOffNameData
	if nameStart+int(nameLen) > len(h.data) {
		return nil, fmt.Errorf("regf: VK name overflows at 0x%x", off)
	}
	var name string
	if nameLen == 0 {
		name = "(Default)"
	} else if flags&0x0001 != 0 {
		// Compressed (ASCII) name
		name = string(h.data[nameStart : nameStart+int(nameLen)])
	} else {
		name = decodeUTF16LE(h.data[nameStart : nameStart+int(nameLen)])
	}

	return &vkCell{
		name:       name,
		dataType:   dataType,
		dataSize:   dataSize,
		dataOffset: dataOffset,
	}, nil
}

// FindSubkey performs a case-insensitive lookup for a direct subkey of the
// given NK cell. Returns nil if not found.
func (h *Hive) FindSubkey(parent *nkCell, name string) (*nkCell, error) {
	if parent.subkeysListOffset == -1 {
		return nil, nil //nolint:nilnil // nil signals "not found"
	}
	return h.findInSubkeyList(parent.subkeysListOffset, name)
}

// maxSubkeyListDepth caps recursion through ri (index-of-index) structures to
// guard against stack overflow on malformed hives. Real-world hives use at most
// one ri level.
const maxSubkeyListDepth = 10

// findInSubkeyList walks lf/lh/li/ri index structures to locate a named subkey.
func (h *Hive) findInSubkeyList(listOff int32, name string) (*nkCell, error) {
	return h.findInSubkeyListDepth(listOff, name, 0)
}

func (h *Hive) findInSubkeyListDepth(listOff int32, name string, depth int) (*nkCell, error) {
	if depth > maxSubkeyListDepth {
		return nil, fmt.Errorf("regf: subkey list recursion exceeded %d levels", maxSubkeyListDepth)
	}
	abs, sig, count, err := h.readSubkeyListHeader(listOff)
	if err != nil {
		return nil, err
	}

	switch sig {
	case sigLF, sigLH:
		return h.findNKInList(abs, count, 8, name, listOff)
	case sigLI:
		return h.findNKInList(abs, count, 4, name, listOff)
	case sigRI:
		return h.findNKInRI(abs, count, name, depth, listOff)
	default:
		return nil, fmt.Errorf("regf: unknown subkey list signature 0x%04x at 0x%x", sig, listOff)
	}
}

// readSubkeyListHeader validates and reads the header of a subkey list cell.
// Returns the absolute offset past cell-size, signature, and element count.
func (h *Hive) readSubkeyListHeader(listOff int32) (abs int, sig, count uint16, err error) {
	abs = cellOffset(listOff)
	if abs < 0 || abs+subkeyListHeaderSize > len(h.data) {
		return 0, 0, 0, fmt.Errorf("regf: subkey list offset 0x%x out of range", listOff)
	}
	abs += cellSizeLen
	sig = h.readUint16(abs)
	count = h.readUint16(abs + sigLen)
	return abs, sig, count, nil
}

// findNKInList scans an LF/LH/LI element array for a named subkey.
// stride is 8 for LF/LH (offset + hash) or 4 for LI (offset only).
func (h *Hive) findNKInList(abs int, count uint16, stride int, name string, listOff int32) (*nkCell, error) {
	required := 4 + int(count)*stride
	if abs+required > len(h.data) {
		return nil, fmt.Errorf("regf: subkey list overflows at 0x%x (count=%d, stride=%d)", listOff, count, stride)
	}
	for i := 0; i < int(count); i++ {
		nkOff := h.readInt32(abs + 4 + i*stride)
		nk, err := h.parseNK(nkOff)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(nk.name, name) {
			return nk, nil
		}
	}
	return nil, nil //nolint:nilnil // nil signals "not found"
}

// findNKInRI recurses through an RI (index-of-index) structure.
func (h *Hive) findNKInRI(abs int, count uint16, name string, depth int, listOff int32) (*nkCell, error) {
	required := 4 + int(count)*4
	if abs+required > len(h.data) {
		return nil, fmt.Errorf("regf: subkey list overflows at 0x%x (RI count=%d)", listOff, count)
	}
	for i := 0; i < int(count); i++ {
		childListOff := h.readInt32(abs + 4 + i*4)
		nk, err := h.findInSubkeyListDepth(childListOff, name, depth+1)
		if err != nil {
			return nil, err
		}
		if nk != nil {
			return nk, nil
		}
	}
	return nil, nil //nolint:nilnil // nil signals "not found"
}

// EnumerateSubkeys returns all direct child NK cells of the parent.
func (h *Hive) EnumerateSubkeys(parent *nkCell) ([]*nkCell, error) {
	if parent.subkeysListOffset == -1 {
		return nil, nil
	}
	return h.collectSubkeys(parent.subkeysListOffset)
}

func (h *Hive) collectSubkeys(listOff int32) ([]*nkCell, error) {
	return h.collectSubkeysDepth(listOff, 0)
}

func (h *Hive) collectSubkeysDepth(listOff int32, depth int) ([]*nkCell, error) {
	if depth > maxSubkeyListDepth {
		return nil, fmt.Errorf("regf: subkey list recursion exceeded %d levels", maxSubkeyListDepth)
	}
	abs, sig, count, err := h.readSubkeyListHeader(listOff)
	if err != nil {
		return nil, err
	}

	switch sig {
	case sigLF, sigLH:
		return h.collectNKsFromList(abs, count, 8, listOff)
	case sigLI:
		return h.collectNKsFromList(abs, count, 4, listOff)
	case sigRI:
		return h.collectNKsFromRI(abs, count, depth, listOff)
	default:
		return nil, fmt.Errorf("regf: unknown subkey list signature 0x%04x", sig)
	}
}

// collectNKsFromList parses all NK cells from an LF/LH/LI element array.
func (h *Hive) collectNKsFromList(abs int, count uint16, stride int, listOff int32) ([]*nkCell, error) {
	required := 4 + int(count)*stride
	if abs+required > len(h.data) {
		return nil, fmt.Errorf("regf: subkey list overflows at 0x%x (count=%d, stride=%d)", listOff, count, stride)
	}
	result := make([]*nkCell, 0, count)
	for i := 0; i < int(count); i++ {
		nkOff := h.readInt32(abs + 4 + i*stride)
		nk, err := h.parseNK(nkOff)
		if err != nil {
			return nil, err
		}
		result = append(result, nk)
	}
	return result, nil
}

// collectNKsFromRI recurses through an RI structure collecting all NK cells.
func (h *Hive) collectNKsFromRI(abs int, count uint16, depth int, listOff int32) ([]*nkCell, error) {
	required := 4 + int(count)*4
	if abs+required > len(h.data) {
		return nil, fmt.Errorf("regf: subkey list overflows at 0x%x (RI count=%d)", listOff, count)
	}
	var result []*nkCell
	for i := 0; i < int(count); i++ {
		childListOff := h.readInt32(abs + 4 + i*4)
		children, err := h.collectSubkeysDepth(childListOff, depth+1)
		if err != nil {
			return nil, err
		}
		result = append(result, children...)
	}
	return result, nil
}

// FindValue performs a case-insensitive lookup for a named value in the NK cell.
func (h *Hive) FindValue(nk *nkCell, name string) (*vkCell, error) {
	if nk.valuesCount == 0 || nk.valuesListOffset == -1 {
		return nil, nil //nolint:nilnil // nil signals "not found"
	}
	abs := cellOffset(nk.valuesListOffset)
	requiredSize := 4 + int(nk.valuesCount)*4 // cell size + array of VK offsets
	if abs < 0 || abs+requiredSize > len(h.data) {
		return nil, fmt.Errorf("regf: values list offset 0x%x out of range", nk.valuesListOffset)
	}
	abs += 4 // skip cell size
	for i := 0; i < int(nk.valuesCount); i++ {
		vkOff := h.readInt32(abs + i*4)
		vk, err := h.parseVK(vkOff)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(vk.name, name) {
			return vk, nil
		}
	}
	return nil, nil //nolint:nilnil // nil signals "not found"
}

// ReadDWORD reads a REG_DWORD value. Returns (value, true) if found, or
// (0, false) if the value does not exist.
func (h *Hive) ReadDWORD(nk *nkCell, name string) (uint32, bool, error) {
	vk, err := h.FindValue(nk, name)
	if err != nil {
		return 0, false, err
	}
	if vk == nil {
		return 0, false, nil
	}
	if vk.dataType != regDWORD {
		return 0, false, fmt.Errorf("regf: value %q is type %d, expected DWORD", name, vk.dataType)
	}
	// Inline data when dataSize high bit is set or size <= 4
	realSize := vk.dataSize & 0x7FFFFFFF
	if vk.dataSize&0x80000000 != 0 || realSize <= 4 {
		return vk.dataOffset, true, nil
	}
	dataAbs := cellOffset(int32(vk.dataOffset)) + 4
	if dataAbs+4 > len(h.data) {
		return 0, false, fmt.Errorf("regf: DWORD data offset out of range")
	}
	return h.readUint32(dataAbs), true, nil
}

// ReadSZ reads a REG_SZ or REG_EXPAND_SZ value, returning the string.
func (h *Hive) ReadSZ(nk *nkCell, name string) (string, bool, error) {
	vk, err := h.FindValue(nk, name)
	if err != nil {
		return "", false, err
	}
	if vk == nil {
		return "", false, nil
	}
	if vk.dataType != regSZ && vk.dataType != regExpandSZ {
		return "", false, fmt.Errorf("regf: value %q is type %d, expected SZ/EXPAND_SZ", name, vk.dataType)
	}
	realSize := vk.dataSize & 0x7FFFFFFF
	if realSize == 0 {
		return "", true, nil
	}
	// Inline data when high bit is set and size <= 4
	if vk.dataSize&0x80000000 != 0 && realSize <= 4 {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, vk.dataOffset)
		return decodeUTF16LE(buf[:realSize]), true, nil
	}
	dataAbs := cellOffset(int32(vk.dataOffset)) + 4
	if dataAbs+int(realSize) > len(h.data) {
		return "", false, fmt.Errorf("regf: SZ data overflows at 0x%x", vk.dataOffset)
	}
	raw := h.data[dataAbs : dataAbs+int(realSize)]
	s := decodeUTF16LE(raw)
	s = strings.TrimRight(s, "\x00")
	return s, true, nil
}

// ReadBinary reads a REG_BINARY value, returning the raw bytes.
func (h *Hive) ReadBinary(nk *nkCell, name string) ([]byte, bool, error) {
	vk, err := h.FindValue(nk, name)
	if err != nil {
		return nil, false, err
	}
	if vk == nil {
		return nil, false, nil
	}
	if vk.dataType != regBinary {
		return nil, false, fmt.Errorf("regf: value %q is type %d, expected BINARY", name, vk.dataType)
	}
	realSize := vk.dataSize & 0x7FFFFFFF
	if realSize == 0 {
		return nil, true, nil
	}
	// Bit 31 set means data is stored inline in the dataOffset field (REGF resident-data optimization).
	if vk.dataSize&0x80000000 != 0 && realSize <= 4 {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, vk.dataOffset)
		return buf[:realSize], true, nil
	}
	dataAbs := cellOffset(int32(vk.dataOffset)) + 4
	if dataAbs+int(realSize) > len(h.data) {
		return nil, false, fmt.Errorf("regf: BINARY data overflows at 0x%x", vk.dataOffset)
	}
	result := make([]byte, realSize)
	copy(result, h.data[dataAbs:dataAbs+int(realSize)])
	return result, true, nil
}

// ReadMultiSZ reads a REG_MULTI_SZ value, returning the list of strings.
func (h *Hive) ReadMultiSZ(nk *nkCell, name string) ([]string, bool, error) {
	vk, err := h.FindValue(nk, name)
	if err != nil {
		return nil, false, err
	}
	if vk == nil {
		return nil, false, nil
	}
	if vk.dataType != regMultiSZ {
		return nil, false, fmt.Errorf("regf: value %q is type %d, expected MULTI_SZ", name, vk.dataType)
	}
	realSize := vk.dataSize & 0x7FFFFFFF
	if realSize == 0 {
		return nil, true, nil
	}
	// Bit 31 set means data is stored inline in the dataOffset field (REGF resident-data optimization).
	var raw []byte
	if vk.dataSize&0x80000000 != 0 && realSize <= 4 {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, vk.dataOffset)
		raw = buf[:realSize]
	} else {
		dataAbs := cellOffset(int32(vk.dataOffset)) + 4
		if dataAbs+int(realSize) > len(h.data) {
			return nil, false, fmt.Errorf("regf: MULTI_SZ data overflows at 0x%x", vk.dataOffset)
		}
		raw = h.data[dataAbs : dataAbs+int(realSize)]
	}
	decoded := decodeUTF16LE(raw)
	// Split on null characters; the last entry is an empty string from the
	// double-null terminator.
	parts := strings.Split(decoded, "\x00")
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result, true, nil
}

// RootKey returns the root NK cell of the hive.
func (h *Hive) RootKey() (*nkCell, error) {
	return h.parseNK(h.rootCell)
}

// OpenKey traverses a backslash-separated path from the root and returns
// the NK cell at the end. Returns nil if any component is not found.
func (h *Hive) OpenKey(path string) (*nkCell, error) {
	parts := strings.Split(path, "\\")
	root, err := h.RootKey()
	if err != nil {
		return nil, err
	}
	current := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		child, err := h.FindSubkey(current, part)
		if err != nil {
			return nil, err
		}
		if child == nil {
			return nil, nil //nolint:nilnil // nil signals "key not found"
		}
		current = child
	}
	return current, nil
}

// ResolveCurrentControlSet reads SYSTEM\Select\Current to determine which
// ControlSet is active and returns its name (e.g. "ControlSet001").
func (h *Hive) ResolveCurrentControlSet() (string, error) {
	selectKey, err := h.OpenKey("Select")
	if err != nil {
		return "", fmt.Errorf("regf: failed to open Select key: %w", err)
	}
	if selectKey == nil {
		return "", fmt.Errorf("regf: Select key not found in hive")
	}
	current, found, err := h.ReadDWORD(selectKey, "Current")
	if err != nil {
		return "", fmt.Errorf("regf: failed to read Select\\Current: %w", err)
	}
	if !found {
		return "", fmt.Errorf("regf: Select\\Current value not found")
	}
	return fmt.Sprintf("ControlSet%03d", current), nil
}

// decodeUTF16LE decodes a byte slice of UTF-16LE data to a Go string.
func decodeUTF16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u16))
}
