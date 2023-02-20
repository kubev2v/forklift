package base

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	auth "k8s.io/api/authentication/v1"
	auth2 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Default auth provider.
var DefaultAuth = Auth{
	TTL: time.Second * 10,
}

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

// Authenticate token.
func (r *Auth) Permit(ctx *gin.Context, p *api.Provider) (status int, err error) {
	r.mutex.Lock()
	ns := ""
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
	if p.ObjectMeta.UID == "" {
		q := ctx.Request.URL.Query()
		ns = q.Get(NsParam)
	}
	allowed, err := r.permit(token, ns, p)
	if allowed && err != nil {
		log.Error(err, "Authorization failed.")
		status = http.StatusInternalServerError
		return
	}
	if allowed {
		r.cache[key] = time.Now()
	} else {
		status = http.StatusForbidden
		delete(r.cache, token)
		log.Info(
			http.StatusText(status),
			"token",
			token)
	}

	return
}

// Authenticate token.
func (r *Auth) permit(token string, ns string, p *api.Provider) (allowed bool, err error) {
	allowed = true
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
	extra := map[string]auth2.ExtraValue{}
	for k, v := range user.Extra {
		extra[k] = append(
			auth2.ExtraValue{},
			v...)
	}
	// Users should be able to query information on providers from the inventory
	// only if they have permissions for list/get 'providers' in the K8s API
	group, resource, err := api.GetGroupResource(p)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	var verb, namespace string
	if p.ObjectMeta.UID != "" {
		verb = "get"
		namespace = p.Namespace
	} else {
		verb = "list"
		namespace = ns
	}
	review := &auth2.SubjectAccessReview{
		Spec: auth2.SubjectAccessReviewSpec{
			ResourceAttributes: &auth2.ResourceAttributes{
				Group:     group,
				Resource:  resource,
				Namespace: namespace,
				Name:      p.Name,
				Verb:      verb,
			},
			Extra:  extra,
			Groups: user.Groups,
			User:   user.Username,
			UID:    user.UID,
		},
	}
	err = w.Create(context.TODO(), review)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if allowed = review.Status.Allowed; !allowed {
		groupResource := &schema.GroupResource{
			Resource: resource,
			Group:    group,
		}
		err = fmt.Errorf("%s is forbidden: User %q cannot %s resource %q in API group %q in the namespace %q",
			groupResource, user.Username, verb, resource, group, namespace)
	}
	return
}

// Extract token.
func (r *Auth) token(ctx *gin.Context) (token string) {
	header := ctx.GetHeader("Authorization")
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[0] == "Bearer" {
		token = fields[1]
	}

	return
}

// Prune the cache.
// Evacuate expired tokens.
func (r *Auth) prune() {
	for token, t := range r.cache {
		if time.Since(t) > r.TTL {
			delete(r.cache, token)
		}
	}
}

// Cache key.
func (r *Auth) key(token string, p *api.Provider) string {
	return path.Join(
		token,
		p.Namespace,
		p.Name)
}

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
