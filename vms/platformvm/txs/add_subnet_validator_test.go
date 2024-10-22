// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/snowtest"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

// TODO use table tests here
func TestAddSupernetValidatorTxSyntacticVerify(t *testing.T) {
	require := require.New(t)
	clk := mockable.Clock{}
	ctx := snowtest.Context(t, snowtest.PChainID)
	signers := [][]*secp256k1.PrivateKey{preFundedKeys}

	var (
		stx                  *Tx
		addSupernetValidatorTx *AddSupernetValidatorTx
		err                  error
	)

	// Case : signed tx is nil
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, ErrNilSignedTx)

	// Case : unsigned tx is nil
	err = addSupernetValidatorTx.SyntacticVerify(ctx)
	require.ErrorIs(err, ErrNilTx)

	validatorWeight := uint64(2022)
	supernetID := ids.ID{'s', 'u', 'b', 'n', 'e', 't', 'I', 'D'}
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
	addSupernetValidatorTx = &AddSupernetValidatorTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    ctx.NetworkID,
			BlockchainID: ctx.ChainID,
			Ins:          inputs,
			Outs:         outputs,
			Memo:         []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}},
		SupernetValidator: SupernetValidator{
			Validator: Validator{
				NodeID: ctx.NodeID,
				Start:  uint64(clk.Time().Unix()),
				End:    uint64(clk.Time().Add(time.Hour).Unix()),
				Wght:   validatorWeight,
			},
			Supernet: supernetID,
		},
		SupernetAuth: supernetAuth,
	}

	// Case: valid tx
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	require.NoError(stx.SyntacticVerify(ctx))

	// Case: Wrong network ID
	addSupernetValidatorTx.SyntacticallyVerified = false
	addSupernetValidatorTx.NetworkID++
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, avax.ErrWrongNetworkID)
	addSupernetValidatorTx.NetworkID--

	// Case: Specifies primary network SupernetID
	addSupernetValidatorTx.SyntacticallyVerified = false
	addSupernetValidatorTx.Supernet = ids.Empty
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, errAddPrimaryNetworkValidator)
	addSupernetValidatorTx.Supernet = supernetID

	// Case: No weight
	addSupernetValidatorTx.SyntacticallyVerified = false
	addSupernetValidatorTx.Wght = 0
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, ErrWeightTooSmall)
	addSupernetValidatorTx.Wght = validatorWeight

	// Case: Supernet auth indices not unique
	addSupernetValidatorTx.SyntacticallyVerified = false
	input := addSupernetValidatorTx.SupernetAuth.(*secp256k1fx.Input)
	oldInput := *input
	input.SigIndices[0] = input.SigIndices[1]
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, secp256k1fx.ErrInputIndicesNotSortedUnique)
	*input = oldInput

	// Case: adding to Primary Network
	addSupernetValidatorTx.SyntacticallyVerified = false
	addSupernetValidatorTx.Supernet = constants.PrimaryNetworkID
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	err = stx.SyntacticVerify(ctx)
	require.ErrorIs(err, errAddPrimaryNetworkValidator)
}

func TestAddSupernetValidatorMarshal(t *testing.T) {
	require := require.New(t)
	clk := mockable.Clock{}
	ctx := snowtest.Context(t, snowtest.PChainID)
	signers := [][]*secp256k1.PrivateKey{preFundedKeys}

	var (
		stx                  *Tx
		addSupernetValidatorTx *AddSupernetValidatorTx
		err                  error
	)

	// create a valid tx
	validatorWeight := uint64(2022)
	supernetID := ids.ID{'s', 'u', 'b', 'n', 'e', 't', 'I', 'D'}
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
	addSupernetValidatorTx = &AddSupernetValidatorTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    ctx.NetworkID,
			BlockchainID: ctx.ChainID,
			Ins:          inputs,
			Outs:         outputs,
			Memo:         []byte{1, 2, 3, 4, 5, 6, 7, 8},
		}},
		SupernetValidator: SupernetValidator{
			Validator: Validator{
				NodeID: ctx.NodeID,
				Start:  uint64(clk.Time().Unix()),
				End:    uint64(clk.Time().Add(time.Hour).Unix()),
				Wght:   validatorWeight,
			},
			Supernet: supernetID,
		},
		SupernetAuth: supernetAuth,
	}

	// Case: valid tx
	stx, err = NewSigned(addSupernetValidatorTx, Codec, signers)
	require.NoError(err)
	require.NoError(stx.SyntacticVerify(ctx))

	txBytes, err := Codec.Marshal(CodecVersion, stx)
	require.NoError(err)

	parsedTx, err := Parse(Codec, txBytes)
	require.NoError(err)

	require.NoError(parsedTx.SyntacticVerify(ctx))
	require.Equal(stx, parsedTx)
}

func TestAddSupernetValidatorTxNotValidatorTx(t *testing.T) {
	txIntf := any((*AddSupernetValidatorTx)(nil))
	_, ok := txIntf.(ValidatorTx)
	require.False(t, ok)
}

func TestAddSupernetValidatorTxNotDelegatorTx(t *testing.T) {
	txIntf := any((*AddSupernetValidatorTx)(nil))
	_, ok := txIntf.(DelegatorTx)
	require.False(t, ok)
}

func TestAddSupernetValidatorTxNotPermissionlessStaker(t *testing.T) {
	txIntf := any((*AddSupernetValidatorTx)(nil))
	_, ok := txIntf.(PermissionlessStaker)
	require.False(t, ok)
}
