package base

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchStatus applies only the caller's status changes and detects concurrent
// updates made by another reconciler.
func PatchStatus(ctx context.Context, writer client.StatusWriter, original, object client.Object) error {
	return writer.Patch(ctx, object, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
}
