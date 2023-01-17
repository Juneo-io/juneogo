// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/june"
)

func TestBaseTxMarshalJSON(t *testing.T) {
	blockchainID := ids.ID{1}
	utxoTxID := ids.ID{2}
	assetID := ids.ID{3}
	fxID := ids.ID{4}
	tx := &BaseTx{BaseTx: june.BaseTx{
		BlockchainID: blockchainID,
		NetworkID:    4,
		Ins: []*june.TransferableInput{
			{
				FxID:   fxID,
				UTXOID: june.UTXOID{TxID: utxoTxID, OutputIndex: 5},
				Asset:  june.Asset{ID: assetID},
				In:     &june.TestTransferable{Val: 100},
			},
		},
		Outs: []*june.TransferableOutput{
			{
				FxID:  fxID,
				Asset: june.Asset{ID: assetID},
				Out:   &june.TestTransferable{Val: 100},
			},
		},
		Memo: []byte{1, 2, 3},
	}}

	txBytes, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	asString := string(txBytes)
	switch {
	case !strings.Contains(asString, `"networkID":4`):
		t.Fatal("should have network ID")
	case !strings.Contains(asString, `"blockchainID":"SYXsAycDPUu4z2ZksJD5fh5nTDcH3vCFHnpcVye5XuJ2jArg"`):
		t.Fatal("should have blockchainID ID")
	case !strings.Contains(asString, `"inputs":[{"txID":"t64jLxDRmxo8y48WjbRALPAZuSDZ6qPVaaeDzxHA4oSojhLt","outputIndex":5,"assetID":"2KdbbWvpeAShCx5hGbtdF15FMMepq9kajsNTqVvvEbhiCRSxU","fxID":"2mB8TguRrYvbGw7G2UBqKfmL8osS7CfmzAAHSzuZK8bwpRKdY","input":{"Err":null,"Val":100}}]`):
		t.Fatal("inputs are wrong")
	case !strings.Contains(asString, `"outputs":[{"assetID":"2KdbbWvpeAShCx5hGbtdF15FMMepq9kajsNTqVvvEbhiCRSxU","fxID":"2mB8TguRrYvbGw7G2UBqKfmL8osS7CfmzAAHSzuZK8bwpRKdY","output":{"Err":null,"Val":100}}]`):
		t.Fatal("outputs are wrong")
	}
}
