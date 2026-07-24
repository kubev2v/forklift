package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// ExplicitBool implements pflag.Value for boolean flags that require an
// explicit value argument (e.g. --flag true / --flag false) instead of
// Cobra's default presence-based semantics (--flag / --flag=false).
//
// Unlike standard BoolVar, this type does NOT implement IsBoolFlag()
// returning true, so pflag always consumes the next argument as the value.
type ExplicitBool struct {
	value *bool
}

func (b *ExplicitBool) String() string {
	if b.value == nil {
		return "false"
	}
	return fmt.Sprintf("%t", *b.value)
}

func (b *ExplicitBool) Set(s string) error {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		*b.value = true
	case "false", "0", "no":
		*b.value = false
	default:
		return fmt.Errorf("invalid boolean value %q (use true/false)", s)
	}
	return nil
}

func (b *ExplicitBool) Type() string {
	return "bool"
}

// IsBoolFlag returns false so pflag consumes the next token as a value,
// enabling --flag true and --flag false (space-separated).
func (b *ExplicitBool) IsBoolFlag() bool {
	return false
}

// ExplicitBoolVar registers a boolean flag that requires an explicit value.
// Usage: flags.ExplicitBoolVar(cmd.Flags(), &myVar, "flag-name", true, "description")
func ExplicitBoolVar(fs *pflag.FlagSet, p *bool, name string, value bool, usage string) {
	*p = value
	fs.Var(&ExplicitBool{value: p}, name, usage)
}
