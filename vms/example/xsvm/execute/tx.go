// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package execute

import (
	"context"
	"errors"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/engine/snowman/block"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/hashing"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/state"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/tx"
	"github.com/Juneo-io/juneogo/vms/platformvm/warp"
)

const (
	QuorumNumerator   = 2
	QuorumDenominator = 3
)

var (
	_ tx.Visitor = (*Tx)(nil)

	errFeeTooHigh          = errors.New("fee too high")
	errWrongChainID        = errors.New("wrong chainID")
	errMissingBlockContext = errors.New("missing block context")
	errDuplicateImport     = errors.New("duplicate import")
)

type Tx struct {
	Context      context.Context
	ChainContext *snow.Context
	Database     database.KeyValueReaderWriterDeleter

	SkipVerify   bool
	BlockContext *block.Context

	TxID        ids.ID
	Sender      ids.ShortID
	TransferFee uint64
	ExportFee   uint64
	ImportFee   uint64
}

func (t *Tx) Transfer(tf *tx.Transfer) error {
	if tf.MaxFee < t.TransferFee {
		return errFeeTooHigh
	}
	if tf.ChainID != t.ChainContext.ChainID {
		return errWrongChainID
	}

	return utils.Err(
		state.IncrementNonce(t.Database, t.Sender, tf.Nonce),
		state.DecreaseBalance(t.Database, t.Sender, tf.ChainID, t.TransferFee),
		state.DecreaseBalance(t.Database, t.Sender, tf.AssetID, tf.Amount),
		state.IncreaseBalance(t.Database, tf.To, tf.AssetID, tf.Amount),
	)
}

func (t *Tx) Export(e *tx.Export) error {
	if e.MaxFee < t.ExportFee {
		return errFeeTooHigh
	}
	if e.ChainID != t.ChainContext.ChainID {
		return errWrongChainID
	}

	payload, err := tx.NewPayload(
		t.Sender,
		e.Nonce,
		e.IsReturn,
		e.Amount,
		e.To,
	)
	if err != nil {
		return err
	}

	message, err := warp.NewUnsignedMessage(
		t.ChainContext.NetworkID,
		e.ChainID,
		payload.Bytes(),
	)
	if err != nil {
		return err
	}

	var errs wrappers.Errs
	errs.Add(
		state.IncrementNonce(t.Database, t.Sender, e.Nonce),
		state.DecreaseBalance(t.Database, t.Sender, e.ChainID, t.ExportFee),
	)

	if e.IsReturn {
		errs.Add(
			state.DecreaseBalance(t.Database, t.Sender, e.PeerChainID, e.Amount),
		)
	} else {
		errs.Add(
			state.DecreaseBalance(t.Database, t.Sender, e.ChainID, e.Amount),
			state.IncreaseLoan(t.Database, e.PeerChainID, e.Amount),
		)
	}

	errs.Add(
		state.SetMessage(t.Database, t.TxID, message),
	)
	return errs.Err
}

func (t *Tx) Import(i *tx.Import) error {
	if i.MaxFee < t.ImportFee {
		return errFeeTooHigh
	}
	if t.BlockContext == nil {
		return errMissingBlockContext
	}

	message, err := warp.ParseMessage(i.Message)
	if err != nil {
		return err
	}

	var errs wrappers.Errs
	errs.Add(
		state.IncrementNonce(t.Database, t.Sender, i.Nonce),
		state.DecreaseBalance(t.Database, t.Sender, t.ChainContext.ChainID, t.ImportFee),
	)

	payload, err := tx.ParsePayload(message.Payload)
	if err != nil {
		return err
	}

	if payload.IsReturn {
		errs.Add(
			state.IncreaseBalance(t.Database, payload.To, t.ChainContext.ChainID, payload.Amount),
			state.DecreaseLoan(t.Database, message.SourceChainID, payload.Amount),
		)
	} else {
		errs.Add(
			state.IncreaseBalance(t.Database, payload.To, message.SourceChainID, payload.Amount),
		)
	}

	var loanID ids.ID = hashing.ComputeHash256Array(message.UnsignedMessage.Bytes())
	hasLoanID, err := state.HasLoanID(t.Database, message.SourceChainID, loanID)
	if hasLoanID {
		return errDuplicateImport
	}

	errs.Add(
		err,
		state.AddLoanID(t.Database, message.SourceChainID, loanID),
	)

	if t.SkipVerify || errs.Errored() {
		return errs.Err
	}

	return message.Signature.Verify(
		t.Context,
		&message.UnsignedMessage,
		t.ChainContext.NetworkID,
		t.ChainContext.ValidatorState,
		t.BlockContext.PChainHeight,
		QuorumNumerator,
		QuorumDenominator,
	)
}
