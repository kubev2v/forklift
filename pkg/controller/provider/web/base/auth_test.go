package base

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/onsi/gomega"
	auth "k8s.io/api/authentication/v1"
	auth2 "k8s.io/api/authorization/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeWriter struct {
	allowed bool
	trCount int
	arCount int
}

func (r *fakeWriter) Create(
	ctx context.Context,
	object client.Object,
	option ...client.CreateOption) (err error) {
	//
	if tr, cast := object.(*auth.TokenReview); cast {
		tr.Status.Authenticated = r.allowed
		r.trCount++
		return
	}
	if ar, cast := object.(*auth2.SubjectAccessReview); cast {
		ar.Status.Allowed = r.allowed
		r.arCount++
		return
	}

	return
}

func (r *fakeWriter) Delete(
	context.Context,
	client.Object,
	...client.DeleteOption) error {
	//
	return nil
}

func (r *fakeWriter) Update(
	context.Context,
	client.Object,
	...client.UpdateOption) error {
	//
	return nil
}

func (r *fakeWriter) Patch(
	context.Context,
	client.Object,
	client.Patch,
	...client.PatchOption) error {
	//
	return nil
}

func (r *fakeWriter) DeleteAllOf(
	context.Context,
	client.Object,
	...client.DeleteAllOfOption) error {
	//
	return nil
}

func TestAuth(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	ttl := time.Millisecond * 50
	writer := &fakeWriter{allowed: true}
	auth := Auth{
		Writer: writer,
		TTL:    ttl,
	}
	token := "12345"
	ctx := &gin.Context{
		Request: &http.Request{
			Header: map[string][]string{
				"Authorization": {"Bearer " + token},
			},
			URL: &url.URL{},
		},
	}
	provider := &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Namespace: "konveyor-forklift",
			Name:      "test",
		},
	}
	// token.
	g.Expect(auth.Token(ctx)).To(gomega.Equal(token))
	// First call with no cached token.
	status, _ := auth.Permit(ctx, provider)
	g.Expect(auth.cache[token]).ToNot(gomega.BeNil())
	g.Expect(1).To(gomega.Equal(writer.trCount))
	g.Expect(1).To(gomega.Equal(writer.arCount))
	g.Expect(http.StatusOK).To(gomega.Equal(status))
	// Second call with cached token.
	status, _ = auth.Permit(ctx, provider)
	g.Expect(auth.cache[token]).ToNot(gomega.BeNil())
	g.Expect(1).To(gomega.Equal(writer.trCount))
	g.Expect(1).To(gomega.Equal(writer.arCount))
	g.Expect(http.StatusOK).To(gomega.Equal(status))
	// Third call after TTL.
	time.Sleep(ttl)
	status, _ = auth.Permit(ctx, provider)
	g.Expect(auth.cache[token]).ToNot(gomega.BeNil())
	g.Expect(2).To(gomega.Equal(writer.trCount))
	g.Expect(2).To(gomega.Equal(writer.arCount))
	g.Expect(http.StatusOK).To(gomega.Equal(status))
	// Prune
	auth.prune()
	g.Expect(auth.cache[token]).ToNot(gomega.BeNil())
	time.Sleep(ttl * 2)
	auth.prune()
	g.Expect(0).To(gomega.Equal(len(auth.cache)))
}
