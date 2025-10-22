package openstack

import (
	libclient "github.com/kubev2v/forklift/pkg/lib/client/openstack"
)

// Client struct
type Client struct {
	libclient.Client
}
