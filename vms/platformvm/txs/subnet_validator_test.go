// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
)

func TestSupernetValidatorVerifySupernetID(t *testing.T) {
	require := require.New(t)

	// Error path
	{
		vdr := &SupernetValidator{
			Supernet: constants.PrimaryNetworkID,
		}

		require.ErrorIs(vdr.Verify(), errBadSupernetID)
	}

	// Happy path
	{
		vdr := &SupernetValidator{
			Supernet: ids.GenerateTestID(),
			Validator: Validator{
				Wght: 1,
			},
		}

		require.NoError(vdr.Verify())
	}
}
