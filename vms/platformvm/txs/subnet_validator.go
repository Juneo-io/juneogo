// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
)

// SupernetValidator validates a supernet on the Avalanche network.
type SupernetValidator struct {
	Validator `serialize:"true"`

	// ID of the supernet this validator is validating
	Supernet ids.ID `serialize:"true" json:"supernetID"`
}

// SupernetID is the ID of the supernet this validator is validating
func (v *SupernetValidator) SupernetID() ids.ID {
	return v.Supernet
}

// Verify this validator is valid
func (v *SupernetValidator) Verify() error {
	switch v.Supernet {
	case constants.PrimaryNetworkID:
		return errBadSupernetID
	default:
		return v.Validator.Verify()
	}
}
