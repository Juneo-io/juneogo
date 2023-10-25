// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/rpc"
)

var _ StaticClient = (*staticClient)(nil)

// StaticClient for interacting with the JVM static api
type StaticClient interface {
	BuildGenesis(ctx context.Context, args *BuildGenesisArgs, options ...rpc.Option) (*BuildGenesisReply, error)
}

// staticClient is an implementation of a JVM client for interacting with the
// jvm static api
type staticClient struct {
	requester rpc.EndpointRequester
}

// NewClient returns a JVM client for interacting with the jvm static api
func NewStaticClient(uri string) StaticClient {
	return &staticClient{requester: rpc.NewEndpointRequester(
		uri + "/ext/vm/jvm",
	)}
}

func (c *staticClient) BuildGenesis(ctx context.Context, args *BuildGenesisArgs, options ...rpc.Option) (resp *BuildGenesisReply, err error) {
	resp = &BuildGenesisReply{}
	err = c.requester.SendRequest(ctx, "jvm.buildGenesis", args, resp, options...)
	return resp, err
}
