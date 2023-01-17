// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpcchainvm

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/go-plugin"

	"github.com/Juneo-io/juneogo/snow/engine/snowman/block"
	"github.com/Juneo-io/juneogo/version"
	"github.com/Juneo-io/juneogo/vms/rpcchainvm/grpcutils"

	vmpb "github.com/Juneo-io/juneogo/proto/pb/vm"
)

var (
	// Handshake is a common handshake that is shared by plugin and host.
	Handshake = plugin.HandshakeConfig{
		ProtocolVersion:  version.RPCChainVMProtocol,
		MagicCookieKey:   "VM_PLUGIN",
		MagicCookieValue: "dynamic",
	}

	// PluginMap is the map of plugins we can dispense.
	PluginMap = map[string]plugin.Plugin{
		"vm": &vmPlugin{},
	}

	_ plugin.Plugin     = (*vmPlugin)(nil)
	_ plugin.GRPCPlugin = (*vmPlugin)(nil)
)

type vmPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	// Concrete implementation, written in Go. This is only used for plugins
	// that are written in Go.
	vm block.ChainVM
}

// New will be called by the server side of the plugin to pass into the server
// side PluginMap for dispatching.
func New(vm block.ChainVM) plugin.Plugin {
	return &vmPlugin{vm: vm}
}

// GRPCServer registers a new GRPC server.
func (p *vmPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	vmpb.RegisterVMServer(s, NewServer(p.vm))
	return nil
}

// GRPCClient returns a new GRPC client
func (*vmPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return NewClient(vmpb.NewVMClient(c)), nil
}

// Serve serves a ChainVM plugin using sane gRPC server defaults.
func Serve(vm block.ChainVM) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"vm": New(vm),
		},
		// ensure proper defaults
		GRPCServer: grpcutils.NewDefaultServer,
	})
}
