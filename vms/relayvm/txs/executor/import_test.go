// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/database/prefixdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/crypto"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/state"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestNewImportTx(t *testing.T) {
	env := newEnvironment( /*postBanff*/ false)
	defer func() {
		if err := shutdownEnvironment(env); err != nil {
			t.Fatal(err)
		}
	}()

	type test struct {
		description   string
		sourceChainID ids.ID
		sharedMemory  atomic.SharedMemory
		sourceKeys    []*crypto.PrivateKeySECP256K1R
		timestamp     time.Time
		shouldErr     bool
		shouldVerify  bool
	}

	factory := crypto.FactorySECP256K1R{}
	sourceKeyIntf, err := factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	sourceKey := sourceKeyIntf.(*crypto.PrivateKeySECP256K1R)

	cnt := new(byte)

	// Returns a shared memory where GetDatabase returns a database
	// where [recipientKey] has a balance of [amt]
	fundedSharedMemory := func(peerChain ids.ID, assets map[ids.ID]uint64) atomic.SharedMemory {
		*cnt++
		m := atomic.NewMemory(prefixdb.New([]byte{*cnt}, env.baseDB))

		sm := m.NewSharedMemory(env.ctx.ChainID)
		peerSharedMemory := m.NewSharedMemory(peerChain)

		for assetID, amt := range assets {
			// #nosec G404
			utxo := &june.UTXO{
				UTXOID: june.UTXOID{
					TxID:        ids.GenerateTestID(),
					OutputIndex: rand.Uint32(),
				},
				Asset: june.Asset{ID: assetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amt,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Addrs:     []ids.ShortID{sourceKey.PublicKey().Address()},
						Threshold: 1,
					},
				},
			}
			utxoBytes, err := txs.Codec.Marshal(txs.Version, utxo)
			if err != nil {
				t.Fatal(err)
			}
			inputID := utxo.InputID()
			if err := peerSharedMemory.Apply(map[ids.ID]*atomic.Requests{env.ctx.ChainID: {PutRequests: []*atomic.Element{{
				Key:   inputID[:],
				Value: utxoBytes,
				Traits: [][]byte{
					sourceKey.PublicKey().Address().Bytes(),
				},
			}}}}); err != nil {
				t.Fatal(err)
			}
		}

		return sm
	}

	customAssetID := ids.GenerateTestID()

	tests := []test{
		{
			description:   "can't pay fee",
			sourceChainID: env.ctx.AssetChainID,
			sharedMemory: fundedSharedMemory(
				env.ctx.AssetChainID,
				map[ids.ID]uint64{
					env.ctx.JuneAssetID: env.config.TxFee - 1,
				},
			),
			sourceKeys: []*crypto.PrivateKeySECP256K1R{sourceKey},
			shouldErr:  true,
		},
		{
			description:   "can barely pay fee",
			sourceChainID: env.ctx.AssetChainID,
			sharedMemory: fundedSharedMemory(
				env.ctx.AssetChainID,
				map[ids.ID]uint64{
					env.ctx.JuneAssetID: env.config.TxFee,
				},
			),
			sourceKeys:   []*crypto.PrivateKeySECP256K1R{sourceKey},
			shouldErr:    false,
			shouldVerify: true,
		},
		{
			description:   "attempting to import from C-chain",
			sourceChainID: juneChainID,
			sharedMemory: fundedSharedMemory(
				juneChainID,
				map[ids.ID]uint64{
					env.ctx.JuneAssetID: env.config.TxFee,
				},
			),
			sourceKeys:   []*crypto.PrivateKeySECP256K1R{sourceKey},
			timestamp:    env.config.ApricotPhase5Time,
			shouldErr:    false,
			shouldVerify: true,
		},
		{
			description:   "attempting to import non-june from X-chain",
			sourceChainID: env.ctx.AssetChainID,
			sharedMemory: fundedSharedMemory(
				env.ctx.AssetChainID,
				map[ids.ID]uint64{
					env.ctx.JuneAssetID: env.config.TxFee,
					customAssetID:       1,
				},
			),
			sourceKeys:   []*crypto.PrivateKeySECP256K1R{sourceKey},
			timestamp:    env.config.BanffTime,
			shouldErr:    false,
			shouldVerify: true,
		},
	}

	to := ids.GenerateTestShortID()
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			require := require.New(t)

			env.msm.SharedMemory = tt.sharedMemory
			tx, err := env.txBuilder.NewImportTx(
				tt.sourceChainID,
				to,
				tt.sourceKeys,
				ids.ShortEmpty,
			)
			if tt.shouldErr {
				require.Error(err)
				return
			}
			require.NoError(err)

			unsignedTx := tx.Unsigned.(*txs.ImportTx)
			require.NotEmpty(unsignedTx.ImportedInputs)
			require.Equal(len(tx.Creds), len(unsignedTx.Ins)+len(unsignedTx.ImportedInputs), "should have the same number of credentials as inputs")

			totalIn := uint64(0)
			for _, in := range unsignedTx.Ins {
				totalIn += in.Input().Amount()
			}
			for _, in := range unsignedTx.ImportedInputs {
				totalIn += in.Input().Amount()
			}
			totalOut := uint64(0)
			for _, out := range unsignedTx.Outs {
				totalOut += out.Out.Amount()
			}

			require.Equal(env.config.TxFee, totalIn-totalOut, "burned too much")

			fakedState, err := state.NewDiff(lastAcceptedID, env)
			require.NoError(err)

			fakedState.SetTimestamp(tt.timestamp)

			fakedParent := ids.GenerateTestID()
			env.SetState(fakedParent, fakedState)

			verifier := MempoolTxVerifier{
				Backend:       &env.backend,
				ParentID:      fakedParent,
				StateVersions: env,
				Tx:            tx,
			}
			err = tx.Unsigned.Visit(&verifier)
			if tt.shouldVerify {
				require.NoError(err)
			} else {
				require.Error(err)
			}
		})
	}
}
