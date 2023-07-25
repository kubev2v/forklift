package openstack

import (
	libclient "github.com/konveyor/forklift-controller/pkg/lib/client/openstack"
)

// Client struct
type Client struct {
	libclient.Client
}
