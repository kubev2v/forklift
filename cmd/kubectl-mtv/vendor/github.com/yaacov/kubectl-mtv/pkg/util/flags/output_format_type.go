package flags

import (
	"fmt"
	"strings"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// OutputFormatHelp is the help text for the --output / -o flag across all commands.
const OutputFormatHelp = "Output format (table, json, yaml, markdown)"

// QueryHelp is the help text for the --query / -q flag across all commands.
// It highlights the IN operator using square brackets since that is the most common syntax mistake.
const QueryHelp = `Query filter using TSL syntax (e.g. "where name ~= 'prod-.*'", "where name in ['vm1','vm2']")`

// OffloadVendors returns the list of supported storage copy-offload vendor products
// directly from the Forklift API types (single source of truth).
func offloadVendorStrings() []string {
	products := v1beta1.StorageVendorProducts()
	vendors := make([]string, len(products))
	for i, p := range products {
		vendors[i] = string(p)
	}
	return vendors
}

// OffloadVendors is the list of supported offload vendor identifiers,
// derived from kubev2v/forklift StorageVendorProducts().
var OffloadVendors = offloadVendorStrings()

// OffloadVendorHelp is the help text for --default-offload-vendor flags.
var OffloadVendorHelp = "Default offload plugin vendor for storage pairs (" + strings.Join(OffloadVendors, "|") + ")"

// ValidateOffloadVendor returns nil if the vendor is in the supported list, or an error otherwise.
func ValidateOffloadVendor(vendor string) error {
	for _, v := range OffloadVendors {
		if vendor == v {
			return nil
		}
	}
	return fmt.Errorf("must be one of: %s", strings.Join(OffloadVendors, "|"))
}

// OutputFormatTypeFlag implements pflag.Value interface for output format type validation
type OutputFormatTypeFlag struct {
	value        string
	validFormats []string
}

func (o *OutputFormatTypeFlag) String() string {
	return o.value
}

func (o *OutputFormatTypeFlag) Set(value string) error {
	isValid := false
	for _, validType := range o.validFormats {
		if value == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid output format: %s. Valid formats are: %v", value, o.validFormats)
	}

	o.value = value
	return nil
}

func (o *OutputFormatTypeFlag) Type() string {
	return "string"
}

// GetValue returns the output format type value
func (o *OutputFormatTypeFlag) GetValue() string {
	return o.value
}

// GetValidValues returns all valid output format type values for auto-completion
func (o *OutputFormatTypeFlag) GetValidValues() []string {
	return o.validFormats
}

// NewOutputFormatTypeFlag creates a new output format type flag with standard formats
func NewOutputFormatTypeFlag() *OutputFormatTypeFlag {
	return &OutputFormatTypeFlag{
		validFormats: []string{"table", "json", "yaml", "markdown"},
		value:        "table", // default value
	}
}
