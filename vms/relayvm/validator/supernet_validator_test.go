// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
)

func TestSupernetValidatorVerifySupernetID(t *testing.T) {
	require := require.New(t)

	// Error path
	{
		vdr := &SupernetValidator{
			Supernet: constants.PrimaryNetworkID,
		}

		require.Equal(errBadSupernetID, vdr.Verify())
	}

	// Happy path
	{
		vdr := &SupernetValidator{
			Supernet: ids.GenerateTestID(),
			Validator: Validator{
				Wght: 1,
			},
		}

		require.Equal(nil, vdr.Verify())
	}
}
