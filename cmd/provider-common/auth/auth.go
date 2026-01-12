package auth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	authz "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	ForkliftGroup    = "forklift.konveyor.io"
	ProviderResource = "providers"
)

var log = logging.WithName("auth")

func NewProviderAuth(namespace string, name string, verb string, ttl int) *ProviderAuth {
	a := &ProviderAuth{
		TTL:       time.Duration(ttl) * time.Second,
		Namespace: namespace,
		Name:      name,
		Verb:      verb,
		cache:     make(map[string]time.Time),
	}
	return a
}

// ProviderAuth uses a SelfSubjectAccessReview
// to perform user auth related to one specific Provider CR.
type ProviderAuth struct {
	TTL       time.Duration
	mutex     sync.Mutex
	cache     map[string]time.Time
	Verb      string
	Namespace string
	Name      string
}

// Permit determines if the request should be permitted by
// checking that the user (identified by bearer token) has
// permissions on the specified Provider CR.
func (r *ProviderAuth) Permit(ctx *gin.Context) (allowed bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.prune()
	if r.Name == "" || r.Namespace == "" {
		return
	}
	token, ok := r.token(ctx)
	if !ok {
		return
	}
	if t, found := r.cache[token]; found {
		if time.Since(t) <= r.TTL {
			allowed = true
			return
		}
	}
	allowed, err := r.permit(token)
	if err != nil {
		log.Error(err, "Authorization failed.")
		return
	}
	if allowed {
		r.cache[token] = time.Now()
	} else {
		delete(r.cache, token)
	}

	return
}

// Perform an SSAR to determine if the user has access to this provider.
func (r *ProviderAuth) permit(token string) (allowed bool, err error) {
	client, err := r.client(token)
	if err != nil {
		return
	}
	review := &authz.SelfSubjectAccessReview{
		Spec: authz.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authz.ResourceAttributes{
				Group:     ForkliftGroup,
				Resource:  ProviderResource,
				Verb:      r.Verb,
				Namespace: r.Namespace,
				Name:      r.Name,
			},
		},
	}
	err = client.Create(context.TODO(), review)
	if err != nil {
		return
	}
	allowed = review.Status.Allowed
	return
}

// Extract token from auth header.
func (r *ProviderAuth) token(ctx *gin.Context) (token string, ok bool) {
	header := ctx.GetHeader("Authorization")
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[0] == "Bearer" {
		token = fields[1]
		ok = true
	}

	return
}

// Prune the cache.
// Evacuate expired tokens.
func (r *ProviderAuth) prune() {
	for token, t := range r.cache {
		if time.Since(t) > r.TTL {
			delete(r.cache, token)
		}
	}
}

// Build API client with user token.
func (r *ProviderAuth) client(token string) (client k8sclient.Client, err error) {
	var cfg *rest.Config
	cfg, err = config.GetConfig()
	if err != nil {
		return
	}
	cfg.BearerTokenFile = ""
	cfg.BearerToken = token
	client, err = k8sclient.New(
		cfg,
		k8sclient.Options{
			Scheme: scheme.Scheme,
		})
	return
}
