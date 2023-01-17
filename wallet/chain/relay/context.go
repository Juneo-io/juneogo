// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package relay

import (
	stdcontext "context"

	"github.com/Juneo-io/juneogo/api/info"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/jvm"
)

var _ Context = (*context)(nil)

type Context interface {
	NetworkID() uint32
	JuneAssetID() ids.ID
	BaseTxFee() uint64
	CreateSupernetTxFee() uint64
	TransformSupernetTxFee() uint64
	CreateBlockchainTxFee() uint64
	AddPrimaryNetworkValidatorFee() uint64
	AddPrimaryNetworkDelegatorFee() uint64
	AddSupernetValidatorFee() uint64
	AddSupernetDelegatorFee() uint64
}

type context struct {
	networkID                     uint32
	juneAssetID                   ids.ID
	baseTxFee                     uint64
	createSupernetTxFee           uint64
	transformSupernetTxFee        uint64
	createBlockchainTxFee         uint64
	addPrimaryNetworkValidatorFee uint64
	addPrimaryNetworkDelegatorFee uint64
	addSupernetValidatorFee       uint64
	addSupernetDelegatorFee       uint64
}

func NewContextFromURI(ctx stdcontext.Context, uri string) (Context, error) {
	infoClient := info.NewClient(uri)
	assetChainClient := jvm.NewClient(uri, "X")
	return NewContextFromClients(ctx, infoClient, assetChainClient)
}

func NewContextFromClients(
	ctx stdcontext.Context,
	infoClient info.Client,
	assetChainClient jvm.Client,
) (Context, error) {
	networkID, err := infoClient.GetNetworkID(ctx)
	if err != nil {
		return nil, err
	}

	asset, err := assetChainClient.GetAssetDescription(ctx, "JUNE")
	if err != nil {
		return nil, err
	}

	txFees, err := infoClient.GetTxFee(ctx)
	if err != nil {
		return nil, err
	}

	return NewContext(
		networkID,
		asset.AssetID,
		uint64(txFees.TxFee),
		uint64(txFees.CreateSupernetTxFee),
		uint64(txFees.TransformSupernetTxFee),
		uint64(txFees.CreateBlockchainTxFee),
		uint64(txFees.AddPrimaryNetworkValidatorFee),
		uint64(txFees.AddPrimaryNetworkDelegatorFee),
		uint64(txFees.AddSupernetValidatorFee),
		uint64(txFees.AddSupernetDelegatorFee),
	), nil
}

func NewContext(
	networkID uint32,
	juneAssetID ids.ID,
	baseTxFee uint64,
	createSupernetTxFee uint64,
	transformSupernetTxFee uint64,
	createBlockchainTxFee uint64,
	addPrimaryNetworkValidatorFee uint64,
	addPrimaryNetworkDelegatorFee uint64,
	addSupernetValidatorFee uint64,
	addSupernetDelegatorFee uint64,
) Context {
	return &context{
		networkID:                     networkID,
		juneAssetID:                   juneAssetID,
		baseTxFee:                     baseTxFee,
		createSupernetTxFee:           createSupernetTxFee,
		transformSupernetTxFee:        transformSupernetTxFee,
		createBlockchainTxFee:         createBlockchainTxFee,
		addPrimaryNetworkValidatorFee: addPrimaryNetworkValidatorFee,
		addPrimaryNetworkDelegatorFee: addPrimaryNetworkDelegatorFee,
		addSupernetValidatorFee:       addSupernetValidatorFee,
		addSupernetDelegatorFee:       addSupernetDelegatorFee,
	}
}

func (c *context) NetworkID() uint32 {
	return c.networkID
}

func (c *context) JuneAssetID() ids.ID {
	return c.juneAssetID
}

func (c *context) BaseTxFee() uint64 {
	return c.baseTxFee
}

func (c *context) CreateSupernetTxFee() uint64 {
	return c.createSupernetTxFee
}

func (c *context) TransformSupernetTxFee() uint64 {
	return c.transformSupernetTxFee
}

func (c *context) CreateBlockchainTxFee() uint64 {
	return c.createBlockchainTxFee
}

func (c *context) AddPrimaryNetworkValidatorFee() uint64 {
	return c.addPrimaryNetworkValidatorFee
}

func (c *context) AddPrimaryNetworkDelegatorFee() uint64 {
	return c.addPrimaryNetworkDelegatorFee
}

func (c *context) AddSupernetValidatorFee() uint64 {
	return c.addSupernetValidatorFee
}

func (c *context) AddSupernetDelegatorFee() uint64 {
	return c.addSupernetDelegatorFee
}
