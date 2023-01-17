// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	errNoExportOutputs = errors.New("no export outputs")

	_ UnsignedTx             = (*ExportTx)(nil)
	_ secp256k1fx.UnsignedTx = (*ExportTx)(nil)
)

// ExportTx is a transaction that exports an asset to another blockchain.
type ExportTx struct {
	BaseTx `serialize:"true"`

	// Which chain to send the funds to
	DestinationChain ids.ID `serialize:"true" json:"destinationChain"`

	// The outputs this transaction is sending to the other chain
	ExportedOuts []*june.TransferableOutput `serialize:"true" json:"exportedOutputs"`
}

func (t *ExportTx) InitCtx(ctx *snow.Context) {
	for _, out := range t.ExportedOuts {
		out.InitCtx(ctx)
	}
	t.BaseTx.InitCtx(ctx)
}

func (t *ExportTx) ConsumedValue(assetID ids.ID) uint64 {
	value := t.BaseTx.ConsumedValue(assetID)
	for _, out := range t.ExportedOuts {
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

func (t *ExportTx) SyntacticVerify(
	ctx *snow.Context,
	c codec.Manager,
	txFeeAssetID ids.ID,
	txFee uint64,
	_ uint64,
	_ int,
) error {
	switch {
	case t == nil:
		return errNilTx
	case len(t.ExportedOuts) == 0:
		return errNoExportOutputs
	}

	// We don't call [t.BaseTx.SyntacticVerify] because the flow check performed
	// here is more strict than the flow check performed in the [BaseTx].
	// Therefore, we avoid performing a useless flow check by performing the
	// other verifications here.
	if err := t.BaseTx.BaseTx.Verify(ctx); err != nil {
		return err
	}

	return june.VerifyTx(
		txFee,
		txFeeAssetID,
		[][]*june.TransferableInput{t.Ins},
		[][]*june.TransferableOutput{
			t.Outs,
			t.ExportedOuts,
		},
		c,
	)
}

func (t *ExportTx) Visit(v Visitor) error {
	return v.ExportTx(t)
}
