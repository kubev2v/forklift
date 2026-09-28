package nutanix

import (
	"strings"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestImage_With(t *testing.T) {
	m := &model.Image{
		Base:         model.Base{ID: "img-1", Name: "RHEL-8.9-x86_64"},
		ImageUUID:    "img-1",
		ImageType:    "DISK_IMAGE",
		SizeBytes:    10737418240,
		Architecture: "X86_64",
		SourceURI:    "http://example.com/rhel8.qcow2",
	}

	r := &Image{}
	r.With(m)

	if r.ID != m.ID || r.Name != m.Name {
		t.Errorf("expected base fields to be copied, got ID=%q Name=%q", r.ID, r.Name)
	}
	if r.ImageUUID != m.ImageUUID ||
		r.ImageType != m.ImageType ||
		r.SizeBytes != m.SizeBytes ||
		r.Architecture != m.Architecture ||
		r.SourceURI != m.SourceURI {
		t.Errorf("expected With() to copy every model field, got %+v", r)
	}
}

func TestImage_Link(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider-1")}}
	r := &Image{}
	r.ID = "img-1"
	r.Link(provider)

	if !strings.Contains(r.SelfLink, "provider-1") {
		t.Errorf("expected SelfLink to contain the provider UID, got %q", r.SelfLink)
	}
	if !strings.HasSuffix(r.SelfLink, "img-1") {
		t.Errorf("expected SelfLink to end with the image ID, got %q", r.SelfLink)
	}
	if strings.Contains(r.SelfLink, ":") {
		t.Errorf("expected SelfLink to have all route placeholders substituted, got %q", r.SelfLink)
	}
}
