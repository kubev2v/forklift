package plan

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//
// Snapshot object reference.
type SnapshotRef struct {
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`
	Generation int64     `json:"generation"`
}

//
// Source and destination pair.
type SnapshotRefPair struct {
	Source      SnapshotRef `json:"source"`
	Destination SnapshotRef `json:"destination"`
}

//
// Mapping.
type SnapshotMap struct {
	Network SnapshotRef `json:"network"`
	Storage SnapshotRef `json:"storage"`
}

//
// Snapshot
type Snapshot struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// Provider
	Provider SnapshotRefPair `json:"provider"`
	// Plan
	Plan SnapshotRef `json:"plan"`
	// Map.
	Map SnapshotMap `json:"map"`
	// Migration
	Migration SnapshotRef `json:"migration"`
}

//
// Populate the ref using the specified (meta) object.
func (r *SnapshotRef) With(object meta.Object) {
	r.Namespace = object.GetNamespace()
	r.Name = object.GetName()
	r.Generation = object.GetGeneration()
	r.UID = object.GetUID()
}

//
// Match the object and ref by UID/Generation.
func (r *SnapshotRef) Match(object meta.Object) bool {
	return r.UID == object.GetUID() && r.Generation == object.GetGeneration()
}
