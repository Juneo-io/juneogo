// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/database/prefixdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/crypto"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/status"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestAtomicTxImports(t *testing.T) {
	require := require.New(t)

	env := newEnvironment(t)
	env.ctx.Lock.Lock()
	defer func() {
		if err := shutdownEnvironment(env); err != nil {
			t.Fatal(err)
		}
	}()

	utxoID := june.UTXOID{
		TxID:        ids.Empty.Prefix(1),
		OutputIndex: 1,
	}
	amount := uint64(70000)
	recipientKey := preFundedKeys[1]

	m := atomic.NewMemory(prefixdb.New([]byte{5}, env.baseDB))

	env.msm.SharedMemory = m.NewSharedMemory(env.ctx.ChainID)
	peerSharedMemory := m.NewSharedMemory(env.ctx.AssetChainID)
	utxo := &june.UTXO{
		UTXOID: utxoID,
		Asset:  june.Asset{ID: juneAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: amount,
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{recipientKey.PublicKey().Address()},
			},
		},
	}
	utxoBytes, err := txs.Codec.Marshal(txs.Version, utxo)
	require.NoError(err)

	inputID := utxo.InputID()
	err = peerSharedMemory.Apply(map[ids.ID]*atomic.Requests{
		env.ctx.ChainID: {PutRequests: []*atomic.Element{{
			Key:   inputID[:],
			Value: utxoBytes,
			Traits: [][]byte{
				recipientKey.PublicKey().Address().Bytes(),
			},
		}}},
	})
	require.NoError(err)

	tx, err := env.txBuilder.NewImportTx(
		env.ctx.AssetChainID,
		recipientKey.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{recipientKey},
		ids.ShortEmpty, // change addr
	)
	require.NoError(err)

	require.NoError(env.Builder.Add(tx))
	b, err := env.Builder.BuildBlock(context.Background())
	require.NoError(err)
	// Test multiple verify calls work
	require.NoError(b.Verify(context.Background()))
	require.NoError(b.Accept(context.Background()))
	_, txStatus, err := env.state.GetTx(tx.ID())
	require.NoError(err)
	// Ensure transaction is in the committed state
	require.Equal(txStatus, status.Committed)
}
