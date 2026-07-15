package client

import (
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
)

type Client struct {
	*plancontext.Context
}

func (r *Client) Connect() error {
	return nil
}

func (r *Client) Close() {}
