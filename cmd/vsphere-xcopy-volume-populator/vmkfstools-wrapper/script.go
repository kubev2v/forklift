package vmkfstoolswrapper

import _ "embed"

//go:embed vmkfstools_wrapper.sh
var Script []byte

// Version is the expected version of the vmkfstools wrapper script
// This is set at build time via ldflags from version.mk
// If not set via ldflags, defaults to "dev"
var Version = "dev"
