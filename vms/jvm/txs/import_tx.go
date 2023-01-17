// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	errNoImportInputs = errors.New("no import inputs")

	_ UnsignedTx             = (*ImportTx)(nil)
	_ secp256k1fx.UnsignedTx = (*ImportTx)(nil)
)

// ImportTx is a transaction that imports an asset from another blockchain.
type ImportTx struct {
	BaseTx `serialize:"true"`

	// Which chain to consume the funds from
	SourceChain ids.ID `serialize:"true" json:"sourceChain"`

	// The inputs to this transaction
	ImportedIns []*june.TransferableInput `serialize:"true" json:"importedInputs"`
}

func (t *ImportTx) ConsumedValue(assetID ids.ID) uint64 {
	value := uint64(0)
	for _, in := range t.BaseTx.Ins {
		if in.Asset.AssetID() == assetID {
			val, err := math.Add64(value, in.In.Amount())
			if err != nil {
				return uint64(0)
			}
			value = val
		}
	}
	for _, in := range t.ImportedIns {
		if in.Asset.AssetID() == assetID {
			val, err := math.Add64(value, in.In.Amount())
			if err != nil {
				return uint64(0)
			}
			value = val
		}
	}
	for _, out := range t.BaseTx.Outs {
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

// InputUTXOs track which UTXOs this transaction is consuming.
func (t *ImportTx) InputUTXOs() []*june.UTXOID {
	utxos := t.BaseTx.InputUTXOs()
	for _, in := range t.ImportedIns {
		in.Symbol = true
		utxos = append(utxos, &in.UTXOID)
	}
	return utxos
}

// ConsumedAssetIDs returns the IDs of the assets this transaction consumes
func (t *ImportTx) ConsumedAssetIDs() set.Set[ids.ID] {
	assets := t.BaseTx.AssetIDs()
	for _, in := range t.ImportedIns {
		assets.Add(in.AssetID())
	}
	return assets
}

// AssetIDs returns the IDs of the assets this transaction depends on
func (t *ImportTx) AssetIDs() set.Set[ids.ID] {
	assets := t.BaseTx.AssetIDs()
	for _, in := range t.ImportedIns {
		assets.Add(in.AssetID())
	}
	return assets
}

// NumCredentials returns the number of expected credentials
func (t *ImportTx) NumCredentials() int {
	return t.BaseTx.NumCredentials() + len(t.ImportedIns)
}

// SyntacticVerify that this import transaction is well-formed.
func (t *ImportTx) SyntacticVerify(
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
	case len(t.ImportedIns) == 0:
		return errNoImportInputs
	}

	// We don't call [t.BaseTx.SyntacticVerify] because the flow check performed
	// here is less strict than the flow check performed in the [BaseTx].
	if err := t.BaseTx.BaseTx.Verify(ctx); err != nil {
		return err
	}

	return june.VerifyTx(
		txFee,
		txFeeAssetID,
		[][]*june.TransferableInput{
			t.Ins,
			t.ImportedIns,
		},
		[][]*june.TransferableOutput{t.Outs},
		c,
	)
}

func (t *ImportTx) Visit(v Visitor) error {
	return v.ImportTx(t)
}
