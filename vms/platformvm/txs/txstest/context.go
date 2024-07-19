// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txstest

import (
	"time"

	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/vms/platformvm/config"
	"github.com/Juneo-io/juneogo/wallet/chain/p/builder"
)

func newContext(
	ctx *snow.Context,
	cfg *config.Config,
	timestamp time.Time,
) *builder.Context {
	return &builder.Context{
		NetworkID:                     ctx.NetworkID,
		JUNEAssetID:                   ctx.JUNEAssetID,
		BaseTxFee:                     cfg.TxFee,
		CreateSupernetTxFee:             cfg.GetCreateSupernetTxFee(timestamp),
		TransformSupernetTxFee:          cfg.TransformSupernetTxFee,
		CreateBlockchainTxFee:         cfg.GetCreateBlockchainTxFee(timestamp),
		AddPrimaryNetworkValidatorFee: cfg.AddPrimaryNetworkValidatorFee,
		AddPrimaryNetworkDelegatorFee: cfg.AddPrimaryNetworkDelegatorFee,
		AddSupernetValidatorFee:         cfg.AddSupernetValidatorFee,
		AddSupernetDelegatorFee:         cfg.AddSupernetDelegatorFee,
	}
}
