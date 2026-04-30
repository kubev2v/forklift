package conversion

import (
	"context"
	liburl "net/url"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"
)

// GovmomiClientFromProvider builds a logged-in govmomi client using the provider URL and secret,
func GovmomiClientFromProvider(ctx context.Context, provider *api.Provider, secret *core.Secret) (*govmomi.Client, error) {
	if provider == nil {
		return nil, liberr.New("provider is nil")
	}
	if secret == nil {
		return nil, liberr.New("secret is nil")
	}
	urlStr := provider.Spec.URL
	u, err := liburl.Parse(urlStr)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	user := string(secret.Data["user"])
	password := string(secret.Data["password"])
	u.User = liburl.UserPassword(user, password)

	thumbprint := provider.Status.Fingerprint
	skipVerifying := base.GetInsecureSkipVerifyFlag(secret)
	if !skipVerifying {
		cert, errtls := base.VerifyTLSConnection(urlStr, secret)
		if errtls != nil {
			return nil, liberr.Wrap(errtls)
		}
		thumbprint = util.Fingerprint(cert)
	}

	soapClient := soap.NewClient(u, skipVerifying)
	soapClient.SetThumbprint(u.Host, thumbprint)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	client := &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = client.Login(ctx, u.User)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	return client, nil
}
