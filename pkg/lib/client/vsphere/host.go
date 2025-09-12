package vsphere

import (
	"context"
	liburl "net/url"
	"time"

	"github.com/kubev2v/forklift/pkg/controller/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/util"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"
)

// ESX Host.
type EsxHost struct {
	// Host url.
	URL string
	// Host secret.
	Secret *core.Secret
	// Host client.
	client *govmomi.Client
	// Finder
	finder *find.Finder
}

// Test the connection.
func (r *EsxHost) TestConnection() (err error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	err = r.connect(ctx)
	if err == nil {
		r.close()
	}
	return
}

// Translate datastore ID.
func (r *EsxHost) DatastoreID(ds *model.Datastore) (id string, err error) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = r.connect(ctx)
	if err != nil {
		return
	}
	defer r.close()
	object, fErr := r.finder.Datastore(ctx, ds.Name)
	if fErr != nil {
		err = liberr.Wrap(fErr)
		return
	}

	id = object.Reference().Value

	return
}

// Build the client and finder.
func (r *EsxHost) connect(ctx context.Context) (err error) {
	if r.client != nil {
		return
	}
	url, err := liburl.Parse(r.URL)
	if err != nil {
		return liberr.Wrap(err)
	}
	url.User = liburl.UserPassword(
		r.user(),
		r.password())

	thumbprint := r.thumbprint()
	skipVerifying := base.GetInsecureSkipVerifyFlag(r.Secret)

	// If thumbprint is not provided, verify the TLS connection to get it.
	if !skipVerifying && thumbprint == "" {
		cert, errtls := base.VerifyTLSConnection(r.URL, r.Secret)
		if errtls != nil {
			return liberr.Wrap(errtls)
		}
		thumbprint = util.Fingerprint(cert)
	}

	soapClient := soap.NewClient(url, skipVerifying)
	soapClient.SetThumbprint(url.Host, thumbprint)

	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	err = r.client.Login(ctx, url.User)
	if err != nil {
		return liberr.Wrap(err)
	}

	r.finder = find.NewFinder(vimClient)

	return nil
}

// Close connections.
func (r *EsxHost) close() {
	if r.client != nil {
		_ = r.client.Logout(context.TODO())
		r.client.CloseIdleConnections()
		r.client = nil
	}
}

// User.
func (r *EsxHost) user() string {
	if user, found := r.Secret.Data["user"]; found {
		return string(user)
	}

	return ""
}

// Password.
func (r *EsxHost) password() string {
	if password, found := r.Secret.Data["password"]; found {
		return string(password)
	}

	return ""
}

// Thumbprint.
func (r *EsxHost) thumbprint() string {
	if password, found := r.Secret.Data["thumbprint"]; found {
		return string(password)
	}

	return ""
}
