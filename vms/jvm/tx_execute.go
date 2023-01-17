// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jvm

import (
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/jvm/txs"
)

var _ txs.Visitor = (*executeTx)(nil)

type executeTx struct {
	tx           *txs.Tx
	batch        database.Batch
	sharedMemory atomic.SharedMemory
	parser       txs.Parser
}

func (et *executeTx) BaseTx(*txs.BaseTx) error {
	return et.batch.Write()
}

func (et *executeTx) ImportTx(t *txs.ImportTx) error {
	utxoIDs := make([][]byte, len(t.ImportedIns))
	for i, in := range t.ImportedIns {
		inputID := in.UTXOID.InputID()
		utxoIDs[i] = inputID[:]
	}
	return et.sharedMemory.Apply(
		map[ids.ID]*atomic.Requests{
			t.SourceChain: {
				RemoveRequests: utxoIDs,
			},
		},
		et.batch,
	)
}

func (et *executeTx) ExportTx(t *txs.ExportTx) error {
	txID := et.tx.ID()

	elems := make([]*atomic.Element, len(t.ExportedOuts))
	codec := et.parser.Codec()
	for i, out := range t.ExportedOuts {
		utxo := &june.UTXO{
			UTXOID: june.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(len(t.Outs) + i),
			},
			Asset: june.Asset{ID: out.AssetID()},
			Out:   out.Out,
		}

		utxoBytes, err := codec.Marshal(txs.CodecVersion, utxo)
		if err != nil {
			return err
		}

		inputID := utxo.InputID()
		elem := &atomic.Element{
			Key:   inputID[:],
			Value: utxoBytes,
		}
		if out, ok := utxo.Out.(june.Addressable); ok {
			elem.Traits = out.Addresses()
		}

		elems[i] = elem
	}

	return et.sharedMemory.Apply(
		map[ids.ID]*atomic.Requests{
			t.DestinationChain: {
				PutRequests: elems,
			},
		},
		et.batch,
	)
}

func (et *executeTx) CreateAssetTx(t *txs.CreateAssetTx) error {
	return et.BaseTx(&t.BaseTx)
}

func (et *executeTx) OperationTx(t *txs.OperationTx) error {
	return et.BaseTx(&t.BaseTx)
}
