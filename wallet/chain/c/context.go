// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package c

import (
	"github.com/Juneo-io/juneogo/api/info"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/vms/avm"

	stdcontext "context"
)

const Alias = "C"

var _ Context = (*context)(nil)

type Context interface {
	NetworkID() uint32
	BlockchainID() ids.ID
	AVAXAssetID() ids.ID
}

type context struct {
	networkID    uint32
	blockchainID ids.ID
	avaxAssetID  ids.ID
}

func NewContextFromURI(ctx stdcontext.Context, uri string) (Context, error) {
	infoClient := info.NewClient(uri)
	xChainClient := avm.NewClient(uri, "X")
	return NewContextFromClients(ctx, infoClient, xChainClient)
}

func NewContextFromClients(
	ctx stdcontext.Context,
	infoClient info.Client,
	xChainClient avm.Client,
) (Context, error) {
	networkID, err := infoClient.GetNetworkID(ctx)
	if err != nil {
		return nil, err
	}

	chainID, err := infoClient.GetBlockchainID(ctx, Alias)
	if err != nil {
		return nil, err
	}

	asset, err := xChainClient.GetAssetDescription(ctx, "JUNE")
	if err != nil {
		return nil, err
	}

	return NewContext(
		networkID,
		chainID,
		asset.AssetID,
	), nil
}

func NewContext(
	networkID uint32,
	blockchainID ids.ID,
	avaxAssetID ids.ID,
) Context {
	return &context{
		networkID:    networkID,
		blockchainID: blockchainID,
		avaxAssetID:  avaxAssetID,
	}
}

func (c *context) NetworkID() uint32 {
	return c.networkID
}

func (c *context) BlockchainID() ids.ID {
	return c.blockchainID
}

func (c *context) AVAXAssetID() ids.ID {
	return c.avaxAssetID
}

func newSnowContext(c Context) (*snow.Context, error) {
	chainID := c.BlockchainID()
	lookup := ids.NewAliaser()
	return &snow.Context{
		NetworkID:   c.NetworkID(),
		SupernetID:    constants.PrimaryNetworkID,
		ChainID:     chainID,
		CChainID:    chainID,
		AVAXAssetID: c.AVAXAssetID(),
		Log:         logging.NoLog{},
		BCLookup:    lookup,
	}, lookup.Alias(chainID, Alias)
}
