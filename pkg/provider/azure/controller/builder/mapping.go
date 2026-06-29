package builder

import (
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/mapping"
)

func (r *Builder) findStorageMapping(diskSKU string) string {
	return mapping.FindStorageClass(r.Map.Storage, diskSKU)
}
