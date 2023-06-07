// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/verify"
)

var (
	_ UnsignedTx = (*RemoveSupernetValidatorTx)(nil)

	errRemovePrimaryNetworkValidator = errors.New("can't remove primary network validator with RemoveSupernetValidatorTx")
)

// Removes a validator from a supernet.
type RemoveSupernetValidatorTx struct {
	BaseTx `serialize:"true"`
	// The node to remove from the supernet.
	NodeID ids.NodeID `serialize:"true" json:"nodeID"`
	// The supernet to remove the node from.
	Supernet ids.ID `serialize:"true" json:"supernetID"`
	// Proves that the issuer has the right to remove the node from the supernet.
	SupernetAuth verify.Verifiable `serialize:"true" json:"supernetAuthorization"`
}

func (tx *RemoveSupernetValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified:
		// already passed syntactic verification
		return nil
	case tx.Supernet == constants.PrimaryNetworkID:
		return errRemovePrimaryNetworkValidator
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := tx.SupernetAuth.Verify(); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *RemoveSupernetValidatorTx) Visit(visitor Visitor) error {
	return visitor.RemoveSupernetValidatorTx(tx)
}
