package version

import (
	"fmt"
	"os"
	"path/filepath"

	vmkfstoolswrapper "github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper"
)

// Version of the binary, set by the build process using ldflags
var Version = "0.0.0"

// Version of the VIB, set by the build process using ldflags
var VibVersion = "0.0.0"

func Get() string {
	return fmt.Sprintf("binary=%s version=%s vib_version=%s vmkfstools_wrapper_version=%s",
		filepath.Base(os.Args[0]), Version, VibVersion, vmkfstoolswrapper.Version)
}
