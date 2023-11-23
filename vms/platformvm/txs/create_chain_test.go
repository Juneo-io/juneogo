// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestUnsignedCreateChainTxVerify(t *testing.T) {
	ctx := snow.DefaultContextTest()
	testSupernet1ID := ids.GenerateTestID()
	testSupernet1ControlKeys := []*secp256k1.PrivateKey{
		preFundedKeys[0],
		preFundedKeys[1],
	}

	type test struct {
		description string
		supernetID    ids.ID
		genesisData []byte
		vmID        ids.ID
		fxIDs       []ids.ID
		chainName   string
		keys        []*secp256k1.PrivateKey
		setup       func(*CreateChainTx) *CreateChainTx
		expectedErr error
	}

	tests := []test{
		{
			description: "tx is nil",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(*CreateChainTx) *CreateChainTx {
				return nil
			},
			expectedErr: ErrNilTx,
		},
		{
			description: "vm ID is empty",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(tx *CreateChainTx) *CreateChainTx {
				tx.VMID = ids.ID{}
				return tx
			},
			expectedErr: errInvalidVMID,
		},
		{
			description: "supernet ID is platform chain's ID",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(tx *CreateChainTx) *CreateChainTx {
				tx.SupernetID = ctx.ChainID
				return tx
			},
			expectedErr: ErrCantValidatePrimaryNetwork,
		},
		{
			description: "chain name is too long",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(tx *CreateChainTx) *CreateChainTx {
				tx.ChainName = string(make([]byte, MaxNameLen+1))
				return tx
			},
			expectedErr: errNameTooLong,
		},
		{
			description: "chain name has invalid character",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(tx *CreateChainTx) *CreateChainTx {
				tx.ChainName = "⌘"
				return tx
			},
			expectedErr: errIllegalNameCharacter,
		},
		{
			description: "genesis data is too long",
			supernetID:    testSupernet1ID,
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			setup: func(tx *CreateChainTx) *CreateChainTx {
				tx.GenesisData = make([]byte, MaxGenesisLen+1)
				return tx
			},
			expectedErr: errGenesisTooLong,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			require := require.New(t)

			inputs := []*avax.TransferableInput{{
				UTXOID: avax.UTXOID{
					TxID:        ids.ID{'t', 'x', 'I', 'D'},
					OutputIndex: 2,
				},
				Asset: avax.Asset{ID: ids.ID{'a', 's', 's', 'e', 't'}},
				In: &secp256k1fx.TransferInput{
					Amt:   uint64(5678),
					Input: secp256k1fx.Input{SigIndices: []uint32{0}},
				},
			}}
			outputs := []*avax.TransferableOutput{{
				Asset: avax.Asset{ID: ids.ID{'a', 's', 's', 'e', 't'}},
				Out: &secp256k1fx.TransferOutput{
					Amt: uint64(1234),
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs:     []ids.ShortID{preFundedKeys[0].PublicKey().Address()},
					},
				},
			}}
			supernetAuth := &secp256k1fx.Input{
				SigIndices: []uint32{0, 1},
			}

			createChainTx := &CreateChainTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Ins:          inputs,
					Outs:         outputs,
				}},
				SupernetID:    test.supernetID,
				ChainName:   test.chainName,
				VMID:        test.vmID,
				FxIDs:       test.fxIDs,
				GenesisData: test.genesisData,
				SupernetAuth:  supernetAuth,
			}

			signers := [][]*secp256k1.PrivateKey{preFundedKeys}
			stx, err := NewSigned(createChainTx, Codec, signers)
			require.NoError(err)

			createChainTx.SyntacticallyVerified = false
			stx.Unsigned = test.setup(createChainTx)

			err = stx.SyntacticVerify(ctx)
			require.ErrorIs(err, test.expectedErr)
		})
	}
}
