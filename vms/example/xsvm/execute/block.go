// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package execute

import (
	"context"
	"errors"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/state"

	smblock "github.com/Juneo-io/juneogo/snow/engine/snowman/block"
	xsblock "github.com/Juneo-io/juneogo/vms/example/xsvm/block"
)

var errNoTxs = errors.New("no transactions")

func Block(
	ctx context.Context,
	chainContext *snow.Context,
	db database.KeyValueReaderWriterDeleter,
	skipVerify bool,
	blockContext *smblock.Context,
	blk *xsblock.Stateless,
) error {
	if len(blk.Txs) == 0 {
		return errNoTxs
	}

	for _, currentTx := range blk.Txs {
		txID, err := currentTx.ID()
		if err != nil {
			return err
		}
		sender, err := currentTx.SenderID()
		if err != nil {
			return err
		}
		txExecutor := Tx{
			Context:      ctx,
			ChainContext: chainContext,
			Database:     db,
			SkipVerify:   skipVerify,
			BlockContext: blockContext,
			TxID:         txID,
			Sender:       sender,
			// TODO: populate fees
		}
		if err := currentTx.Unsigned.Visit(&txExecutor); err != nil {
			return err
		}
	}

	blkID, err := blk.ID()
	if err != nil {
		return err
	}

	if err := state.SetLastAccepted(db, blkID); err != nil {
		return err
	}

	blkBytes, err := xsblock.Codec.Marshal(xsblock.Version, blk)
	if err != nil {
		return err
	}

	return state.AddBlock(db, blk.Height, blkID, blkBytes)
}
