// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

var (
	_ StakerTx = (*AddSupernetValidatorTx)(nil)

	errAddPrimaryNetworkValidator = errors.New("can't add primary network validator with AddSupernetValidatorTx")
)

// AddSupernetValidatorTx is an unsigned addSupernetValidatorTx
type AddSupernetValidatorTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// The validator
	SupernetValidator `serialize:"true" json:"validator"`
	// Auth that will be allowing this validator into the network
	SupernetAuth verify.Verifiable `serialize:"true" json:"supernetAuthorization"`
}

func (tx *AddSupernetValidatorTx) NodeID() ids.NodeID {
	return tx.SupernetValidator.NodeID
}

func (*AddSupernetValidatorTx) PublicKey() (*bls.PublicKey, bool, error) {
	return nil, false, nil
}

func (*AddSupernetValidatorTx) PendingPriority() Priority {
	return SupernetPermissionedValidatorPendingPriority
}

func (*AddSupernetValidatorTx) CurrentPriority() Priority {
	return SupernetPermissionedValidatorCurrentPriority
}

// SyntacticVerify returns nil iff [tx] is valid
func (tx *AddSupernetValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.Supernet == constants.PrimaryNetworkID:
		return errAddPrimaryNetworkValidator
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := verify.All(&tx.Validator, tx.SupernetAuth); err != nil {
		return err
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *AddSupernetValidatorTx) Visit(visitor Visitor) error {
	return visitor.AddSupernetValidatorTx(tx)
}
