// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	_ UnsignedTx             = (*BaseTx)(nil)
	_ secp256k1fx.UnsignedTx = (*BaseTx)(nil)
)

// BaseTx is the basis of all transactions.
type BaseTx struct {
	june.BaseTx `serialize:"true"`

	bytes []byte
}

func (t *BaseTx) InitCtx(ctx *snow.Context) {
	for _, out := range t.Outs {
		out.InitCtx(ctx)
	}
}

func (t *BaseTx) SetBytes(bytes []byte) {
	t.bytes = bytes
}

func (t *BaseTx) Bytes() []byte {
	return t.bytes
}

func (t *BaseTx) ConsumedValue(assetID ids.ID) uint64 {
	value := uint64(0)
	for _, in := range t.Ins {
		if in.Asset.AssetID() == assetID {
			val, err := math.Add64(value, in.In.Amount())
			if err != nil {
				return uint64(0)
			}
			value = val
		}
	}
	for _, out := range t.Outs {
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

func (t *BaseTx) SyntacticVerify(
	ctx *snow.Context,
	c codec.Manager,
	txFeeAssetID ids.ID,
	txFee uint64,
	_ uint64,
	_ int,
) error {
	if t == nil {
		return errNilTx
	}

	if err := t.BaseTx.Verify(ctx); err != nil {
		return err
	}

	return june.VerifyTx(
		txFee,
		txFeeAssetID,
		[][]*june.TransferableInput{t.Ins},
		[][]*june.TransferableOutput{t.Outs},
		c,
	)
}

func (t *BaseTx) Visit(v Visitor) error {
	return v.BaseTx(t)
}
