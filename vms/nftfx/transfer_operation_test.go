// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package nftfx

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestTransferOperationVerifyNil(t *testing.T) {
	op := (*TransferOperation)(nil)
	err := op.Verify()
	require.ErrorIs(t, err, errNilTransferOperation)
}

func TestTransferOperationInvalid(t *testing.T) {
	op := TransferOperation{Input: secp256k1fx.Input{
		SigIndices: []uint32{1, 0},
	}}
	err := op.Verify()
	require.ErrorIs(t, err, secp256k1fx.ErrInputIndicesNotSortedUnique)
}

func TestTransferOperationOuts(t *testing.T) {
	op := TransferOperation{
		Output: TransferOutput{},
	}
	require.Len(t, op.Outs(), 1)
}

func TestTransferOperationState(t *testing.T) {
	intf := interface{}(&TransferOperation{})
	_, ok := intf.(verify.State)
	require.False(t, ok)
}
