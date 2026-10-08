package vsphere

import (
	"context"
	"time"

	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
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
	r.client, err = libvsphere.ConnectProvider(ctx, r.URL, r.user(), r.password(), r.thumbprint(), r.Secret)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.finder = find.NewFinder(r.client.Client)
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
