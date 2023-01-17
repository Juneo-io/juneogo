// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package teleporter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
)

func TestSigner(t *testing.T) {
	require := require.New(t)

	for _, test := range SignerTests {
		sk, err := bls.NewSecretKey()
		require.NoError(err)

		chainID := ids.GenerateTestID()
		s := NewSigner(sk, chainID)

		test(t, s, sk, chainID)
	}
}
