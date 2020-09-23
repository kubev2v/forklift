package v1alpha1

import (
	"context"
	"encoding/json"
	"errors"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/ref"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

//
// Errors.
var (
	NotFoundInSnapshotErr = errors.New("not found in snapshot")
)

//
// Owner.
type Resource interface {
	meta.Object
	runtime.Object
}

//
// Snapshot
// +k8s:deepcopy-gen=false
type Snapshot struct {
	core.ConfigMap
	// Owner resource.
	Owner Resource
}

func (s *Snapshot) DeepCopyInto(*Snapshot) {
}

func (s *Snapshot) DeepCopy() *Snapshot {
	return s
}

//
// Read() did not find the backing map.
func (s *Snapshot) NotFound() bool {
	return s.UID == ""
}

//
// Build the backing map.
func (s *Snapshot) BuildMap() *core.ConfigMap {
	gvk := s.Owner.GetObjectKind().GroupVersionKind()
	return &core.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Namespace: s.Owner.GetNamespace(),
			Name:      s.mapName(),
			OwnerReferences: []meta.OwnerReference{
				{
					APIVersion: gvk.Version,
					Kind:       gvk.Kind,
					Name:       s.Owner.GetName(),
					UID:        s.Owner.GetUID(),
				},
			},
		},
		Data: map[string]string{},
	}
}

//
// Read the backing map.
func (s *Snapshot) Read(c client.Client) (err error) {
	key := client.ObjectKey{
		Namespace: s.Owner.GetNamespace(),
		Name:      s.mapName(),
	}
	err = c.Get(context.TODO(), key, &s.ConfigMap)
	if err == nil {
		return
	}
	if k8serr.IsNotFound(err) {
		s.ConfigMap = *s.BuildMap()
		err = nil
	} else {
		err = liberr.Wrap(err)
	}

	return
}

//
// Create/Update the backing map.
func (s *Snapshot) Write(c client.Client) (err error) {
	if s.NotFound() {
		err = c.Create(context.TODO(), &s.ConfigMap)
		if err == nil {
			return
		}
		if !k8serr.IsAlreadyExists(err) {
			err = liberr.Wrap(err)
			return
		}
	}
	err = c.Update(context.TODO(), &s.ConfigMap)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Get an object from the snapshot.
// Returns NotFoundInSnapshotErr when not found or
// potentially a json error.
func (s *Snapshot) Get(object Resource) (err error) {
	if s.Data == nil {
		err = liberr.Wrap(NotFoundInSnapshotErr)
		return
	}
	key := s.key(object)
	if j, found := s.Data[key]; found {
		err = json.Unmarshal([]byte(j), object)
	} else {
		err = liberr.Wrap(NotFoundInSnapshotErr)
	}

	return
}

//
// Set an object in the snapshot.
func (s *Snapshot) Set(object Resource) {
	if object == nil {
		return
	}
	j, _ := json.Marshal(object)
	key := s.key(object)
	s.Data[key] = string(j)
}

//
// Build object key based on the object:
//  - kind
//  - namespace
//  - name
func (s *Snapshot) key(object Resource) string {
	return strings.Join(
		[]string{
			ref.ToKind(object),
			object.GetNamespace(),
			object.GetName(),
		},
		"-")
}

//
// Build map name based on the owner:
//   - kind
//   - name
//   - uid (last 4).
func (s *Snapshot) mapName() string {
	uid := string(s.Owner.GetUID())
	gvk := s.Owner.GetObjectKind().GroupVersionKind()
	return strings.Join(
		[]string{
			strings.ToLower(gvk.Kind),
			s.Owner.GetName(),
			uid[len(uid)-4:],
		},
		"-")
}
