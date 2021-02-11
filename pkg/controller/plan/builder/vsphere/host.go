package vsphere

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	core "k8s.io/api/core/v1"
	liburl "net/url"
	"time"
)

//
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

//
// Test the connection.
func (r *EsxHost) TestConnection() (err error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	err = r.connect(ctx)
	return
}

//
// Translate network ID.
func (r *EsxHost) networkID(network *model.Network) (id string, err error) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = r.connect(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = r.client.Logout(ctx)
	}()
	object, fErr := r.finder.Network(ctx, network.Name)
	if fErr != nil {
		err = fErr
		return
	}

	id = object.Reference().Value

	return
}

//
// Translate datastore ID.
func (r *EsxHost) DatastoreID(ds *model.Datastore) (id string, err error) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = r.connect(ctx)
	if err != nil {
		return
	}
	defer func() {
		_ = r.client.Logout(ctx)
	}()
	object, fErr := r.finder.Datastore(ctx, ds.Name)
	if fErr != nil {
		err = fErr
		return
	}

	id = object.Reference().Value

	return
}

//
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
	soapClient := soap.NewClient(url, false)
	soapClient.SetThumbprint(url.Host, r.thumbprint())
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

//
// User.
func (r *EsxHost) user() string {
	if user, found := r.Secret.Data["user"]; found {
		return string(user)
	}

	return ""
}

//
// Password.
func (r *EsxHost) password() string {
	if password, found := r.Secret.Data["password"]; found {
		return string(password)
	}

	return ""
}

//
// Thumbprint.
func (r *EsxHost) thumbprint() string {
	if password, found := r.Secret.Data["thumbprint"]; found {
		return string(password)
	}

	return ""
}
