package openstack

import (
	"fmt"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

const (
	forkliftPropertyOriginalVolumeID = "forklift_original_volume_id"
	forkliftPropertyOriginalImageID  = "forklift_original_image_id"
)

func getMigrationID(ctx *plancontext.Context) string {
	return string(ctx.Migration.GetUID())
}

func getVmSnapshotName(ctx *plancontext.Context, vmID string) string {
	const nameFormat = "forklift-migration-vm-%s"
	return fmt.Sprintf(nameFormat, vmID)
}

func getSnapshotFromVolumeName(ctx *plancontext.Context, vmID string) string {
	const nameFormat = "snapshot for %s"
	return fmt.Sprintf(nameFormat, getVmSnapshotName(ctx, vmID))
}

func getVolumeFromSnapshotName(ctx *plancontext.Context, vmID, snapshotID string) string {
	const nameFormat = "volume created from %s"
	return fmt.Sprintf(nameFormat, getVmSnapshotName(ctx, vmID))
}

func getImageFromVolumeName(ctx *plancontext.Context, vmID, volumeID string) string {
	const nameFormat = "%s-volume-%s"
	return fmt.Sprintf(nameFormat, getVmSnapshotName(ctx, vmID), volumeID)
}
