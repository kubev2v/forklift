package vsphere

import (
	"context"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
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
	// Inventory client.
	Inventory web.Client
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
	if err != nil {
		liberr.Wrap(err)
	}

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
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = r.client.Logout(ctx)
	}()
	object, fErr := r.finder.Network(ctx, network.Name)
	if fErr != nil {
		err = liberr.Wrap(fErr)
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
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = r.client.Logout(ctx)
	}()
	object, fErr := r.finder.Datastore(ctx, ds.Name)
	if fErr != nil {
		err = liberr.Wrap(fErr)
		return
	}

	id = object.Reference().Value

	return
}

//
// Build the client and finder.
func (r *EsxHost) connect(ctx context.Context) (err error) {
	insecure := true
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

	r.client, err = govmomi.NewClient(ctx, url, insecure)
	if err != nil {
		return liberr.Wrap(err)
	}
	client, err := vim25.NewClient(ctx, r.client)
	if err != nil {
		err = liberr.Wrap(err)
	}

	r.finder = find.NewFinder(client)

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
