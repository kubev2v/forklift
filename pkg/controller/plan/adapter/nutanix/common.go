package nutanix

import (
	"fmt"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

const (
	forkliftPropertyOriginalDiskUUID = "forklift_original_disk_uuid"
	imageStateComplete               = "COMPLETE"
)

func getExportImageName(_ *plancontext.Context, vmUUID, diskUUID string) string {
	return fmt.Sprintf("forklift-migration-%s-disk-%s", vmUUID, diskUUID)
}
