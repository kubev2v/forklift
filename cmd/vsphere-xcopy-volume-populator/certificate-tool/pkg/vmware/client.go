package vmware

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

func SetupVSphere(timeout time.Duration, vcURL, user, pass, dcName, dsName, poolName string,
) (
	ctx context.Context,
	cancel context.CancelFunc,
	cli *govmomi.Client,
	finder *find.Finder,
	dc *object.Datacenter,
	ds *object.Datastore,
	rp *object.ResourcePool,
	err error,
) {
	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	u, err := url.Parse(vcURL)
	if err != nil {
		err = fmt.Errorf("invalid vCenter URL: %w", err)
		return
	}
	u.User = url.UserPassword(user, pass)
	cli, err = govmomi.NewClient(ctx, u, true /* allowInsecure */)
	if err != nil {
		err = fmt.Errorf("vCenter connect error: %w", err)
		return
	}
	finder = find.NewFinder(cli.Client, false)
	dc, err = finder.Datacenter(ctx, dcName)
	if err != nil {
		err = fmt.Errorf("find datacenter %q: %w", dcName, err)
		return
	}
	finder.SetDatacenter(dc)
	ds, err = finder.Datastore(ctx, dsName)
	if err != nil {
		err = fmt.Errorf("find datastore %q: %w", dsName, err)
		return
	}
	rp, err = finder.ResourcePool(ctx, poolName)
	if err != nil {
		err = fmt.Errorf("find resource pool %q: %w", dsName, err)
		return
	}

	return
}
