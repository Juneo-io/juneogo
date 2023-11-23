// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package nftfx

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestTransferOutputVerifyNil(t *testing.T) {
	to := (*TransferOutput)(nil)
	err := to.Verify()
	require.ErrorIs(t, err, errNilTransferOutput)
}

func TestTransferOutputLargePayload(t *testing.T) {
	to := TransferOutput{
		Payload: make([]byte, MaxPayloadSize+1),
	}
	err := to.Verify()
	require.ErrorIs(t, err, errPayloadTooLarge)
}

func TestTransferOutputInvalidSecp256k1Output(t *testing.T) {
	to := TransferOutput{
		OutputOwners: secp256k1fx.OutputOwners{
			Addrs: []ids.ShortID{
				ids.ShortEmpty,
				ids.ShortEmpty,
			},
		},
	}
	err := to.Verify()
	require.ErrorIs(t, err, secp256k1fx.ErrOutputUnoptimized)
}

func TestTransferOutputState(t *testing.T) {
	intf := interface{}(&TransferOutput{})
	_, ok := intf.(verify.State)
	require.True(t, ok)
}
