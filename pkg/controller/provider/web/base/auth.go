package base

import (
	"context"
	"github.com/gin-gonic/gin"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	auth "k8s.io/api/authentication/v1"
	auth2 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"net/http"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strings"
	"sync"
	"time"
)

//
// Default auth provider.
var DefaultAuth = Auth{
	TTL: time.Second * 10,
}

//
// Authorized by k8s bearer token SAR.
// Token must have "*" on the provider CR.
type Auth struct {
	// k8s API writer.
	Writer client.Writer
	// Cached token TTL.
	TTL time.Duration
	// Mutex.
	mutex sync.Mutex
	// Token cache.
	cache map[string]time.Time
}

//
// Authenticate token.
func (r *Auth) Permit(ctx *gin.Context, p *api.Provider) (status int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	status = http.StatusOK
	if r.cache == nil {
		r.cache = make(map[string]time.Time)
	}
	r.prune()
	token := r.token(ctx)
	if token == "" {
		status = http.StatusUnauthorized
		return
	}
	key := r.key(token, p)
	if t, found := r.cache[key]; found {
		if time.Since(t) <= r.TTL {
			return
		}
	}
	allowed, err := r.permit(token, p)
	if err != nil {
		status = http.StatusInternalServerError
		return
	}
	if allowed {
		r.cache[key] = time.Now()
	} else {
		status = http.StatusForbidden
		delete(r.cache, token)
	}

	return
}

//
// Authenticate token.
func (r *Auth) permit(token string, p *api.Provider) (allowed bool, err error) {
	tr := &auth.TokenReview{
		Spec: auth.TokenReviewSpec{
			Token: token,
		},
	}
	w, err := r.writer()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = w.Create(context.TODO(), tr)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !tr.Status.Authenticated {
		return
	}
	user := tr.Status.User
	kind := p.GetObjectKind()
	gvk := kind.GroupVersionKind()
	ar := &auth2.SubjectAccessReview{
		Spec: auth2.SubjectAccessReviewSpec{
			ResourceAttributes: &auth2.ResourceAttributes{
				Group:     gvk.Group,
				Resource:  gvk.Kind,
				Namespace: p.Namespace,
				Name:      p.Name,
				Verb:      "*",
			},
			Groups: user.Groups,
			User:   user.UID,
		},
	}
	err = w.Create(context.TODO(), ar)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	allowed = ar.Status.Allowed
	return
}

//
// Extract token.
func (r *Auth) token(ctx *gin.Context) (token string) {
	header := ctx.GetHeader("Authorization")
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[0] == "Bearer" {
		token = fields[1]
	}

	return
}

//
// Prune the cache.
// Evacuate expired tokens.
func (r *Auth) prune() {
	for token, t := range r.cache {
		if time.Since(t) > r.TTL {
			delete(r.cache, token)
		}
	}
}

//
// Cache key.
func (r *Auth) key(token string, p *api.Provider) string {
	return path.Join(
		token,
		p.Namespace,
		p.Name)
}

//
// Build API writer.
func (r *Auth) writer() (w client.Writer, err error) {
	if r.Writer != nil {
		w = r.Writer
		return
	}
	cfg, err := config.GetConfig()
	if err != nil {
		return
	}
	cfg.Burst = 1000
	cfg.QPS = 100
	w, err = client.New(
		cfg,
		client.Options{
			Scheme: scheme.Scheme,
		})
	if err == nil {
		r.Writer = w
	}

	return
}
