// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/logging"
)

const Alias = "X"

type Context struct {
	NetworkID        uint32
	BlockchainID     ids.ID
	JUNEAssetID      ids.ID
	BaseTxFee        uint64
	CreateAssetTxFee uint64
}

func NewSnowContext(
	networkID uint32,
	blockchainID ids.ID,
	juneAssetID ids.ID,
) (*snow.Context, error) {
	lookup := ids.NewAliaser()
	return &snow.Context{
		NetworkID:   networkID,
		SupernetID:    constants.PrimaryNetworkID,
		ChainID:     blockchainID,
		JVMChainID:    blockchainID,
		JUNEAssetID: juneAssetID,
		Log:         logging.NoLog{},
		BCLookup:    lookup,
	}, lookup.Alias(blockchainID, Alias)
}
