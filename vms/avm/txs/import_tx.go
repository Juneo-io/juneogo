// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	_ UnsignedTx             = (*ImportTx)(nil)
	_ secp256k1fx.UnsignedTx = (*ImportTx)(nil)
)

// ImportTx is a transaction that imports an asset from another blockchain.
type ImportTx struct {
	BaseTx `serialize:"true"`

	// Which chain to consume the funds from
	SourceChain ids.ID `serialize:"true" json:"sourceChain"`

	// The inputs to this transaction
	ImportedIns []*avax.TransferableInput `serialize:"true" json:"importedInputs"`
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
func (t *ImportTx) InputUTXOs() []*avax.UTXOID {
	utxos := t.BaseTx.InputUTXOs()
	for _, in := range t.ImportedIns {
		in.Symbol = true
		utxos = append(utxos, &in.UTXOID) //nolint:gosec
	}
	return utxos
}

func (t *ImportTx) InputIDs() set.Set[ids.ID] {
	inputs := t.BaseTx.InputIDs()
	for _, in := range t.ImportedIns {
		inputs.Add(in.InputID())
	}
	return inputs
}

// NumCredentials returns the number of expected credentials
func (t *ImportTx) NumCredentials() int {
	return t.BaseTx.NumCredentials() + len(t.ImportedIns)
}

func (t *ImportTx) Visit(v Visitor) error {
	return v.ImportTx(t)
}
