// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/Juneo-io/juneogo/api/info"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/vms/avm"
)

const Alias = "P"

type Context struct {
	NetworkID                     uint32
	AVAXAssetID                   ids.ID
	BaseTxFee                     uint64
	CreateSupernetTxFee             uint64
	TransformSupernetTxFee          uint64
	CreateBlockchainTxFee         uint64
	AddPrimaryNetworkValidatorFee uint64
	AddPrimaryNetworkDelegatorFee uint64
	AddSupernetValidatorFee         uint64
	AddSupernetDelegatorFee         uint64
}

func NewContextFromURI(ctx context.Context, uri string) (*Context, error) {
	infoClient := info.NewClient(uri)
	xChainClient := avm.NewClient(uri, "X")
	return NewContextFromClients(ctx, infoClient, xChainClient)
}

func NewContextFromClients(
	ctx context.Context,
	infoClient info.Client,
	xChainClient avm.Client,
) (*Context, error) {
	networkID, err := infoClient.GetNetworkID(ctx)
	if err != nil {
		return nil, err
	}

	asset, err := xChainClient.GetAssetDescription(ctx, "JUNE")
	if err != nil {
		return nil, err
	}

	txFees, err := infoClient.GetTxFee(ctx)
	if err != nil {
		return nil, err
	}

	return &Context{
		NetworkID:                     networkID,
		AVAXAssetID:                   asset.AssetID,
		BaseTxFee:                     uint64(txFees.TxFee),
		CreateSupernetTxFee:             uint64(txFees.CreateSupernetTxFee),
		TransformSupernetTxFee:          uint64(txFees.TransformSupernetTxFee),
		CreateBlockchainTxFee:         uint64(txFees.CreateBlockchainTxFee),
		AddPrimaryNetworkValidatorFee: uint64(txFees.AddPrimaryNetworkValidatorFee),
		AddPrimaryNetworkDelegatorFee: uint64(txFees.AddPrimaryNetworkDelegatorFee),
		AddSupernetValidatorFee:         uint64(txFees.AddSupernetValidatorFee),
		AddSupernetDelegatorFee:         uint64(txFees.AddSupernetDelegatorFee),
	}, nil
}

func NewSnowContext(networkID uint32, avaxAssetID ids.ID) (*snow.Context, error) {
	lookup := ids.NewAliaser()
	return &snow.Context{
		NetworkID:   networkID,
		SupernetID:    constants.PrimaryNetworkID,
		ChainID:     constants.PlatformChainID,
		AVAXAssetID: avaxAssetID,
		Log:         logging.NoLog{},
		BCLookup:    lookup,
	}, lookup.Alias(constants.PlatformChainID, Alias)
}
