// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/stakeable"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	_ UnsignedTx = (*ExportTx)(nil)

	ErrWrongLocktime   = errors.New("wrong locktime reported")
	errNoExportOutputs = errors.New("no export outputs")
)

// ExportTx is an unsigned exportTx
type ExportTx struct {
	BaseTx `serialize:"true"`

	// Which chain to send the funds to
	DestinationChain ids.ID `serialize:"true" json:"destinationChain"`

	// Outputs that are exported to the chain
	ExportedOutputs []*june.TransferableOutput `serialize:"true" json:"exportedOutputs"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedExportTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *ExportTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, out := range tx.ExportedOutputs {
		out.FxID = secp256k1fx.ID
		out.InitCtx(ctx)
	}
}

func (tx *ExportTx) ConsumedValue(assetID ids.ID) uint64 {
	value := tx.BaseTx.ConsumedValue(assetID)
	for _, out := range tx.ExportedOutputs {
		if out.Asset.AssetID() == assetID {
			val, err := math.Sub(value, out.Out.Amount())
			if err != nil {
				return uint64(0)
			}
			value = val
		}
	}
	return value
}

// SyntacticVerify this transaction is well-formed
func (tx *ExportTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case len(tx.ExportedOutputs) == 0:
		return errNoExportOutputs
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}

	for _, out := range tx.ExportedOutputs {
		if err := out.Verify(); err != nil {
			return fmt.Errorf("output failed verification: %w", err)
		}
		if _, ok := out.Output().(*stakeable.LockOut); ok {
			return ErrWrongLocktime
		}
	}
	if !june.IsSortedTransferableOutputs(tx.ExportedOutputs, Codec) {
		return errOutputsNotSorted
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *ExportTx) Visit(visitor Visitor) error {
	return visitor.ExportTx(tx)
}
