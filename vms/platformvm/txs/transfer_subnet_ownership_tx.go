// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
)

var (
	_ UnsignedTx = (*TransferSupernetOwnershipTx)(nil)

	ErrTransferPermissionlessSupernet = errors.New("cannot transfer ownership of a permissionless supernet")
)

type TransferSupernetOwnershipTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// ID of the supernet this tx is modifying
	Supernet ids.ID `serialize:"true" json:"supernetID"`
	// Proves that the issuer has the right to remove the node from the supernet.
	SupernetAuth verify.Verifiable `serialize:"true" json:"supernetAuthorization"`
	// Who is now authorized to manage this supernet
	Owner fx.Owner `serialize:"true" json:"newOwner"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [TransferSupernetOwnershipTx]. Also sets the [ctx] to the given [vm.ctx] so
// that the addresses can be json marshalled into human readable format
func (tx *TransferSupernetOwnershipTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	tx.Owner.InitCtx(ctx)
}

func (tx *TransferSupernetOwnershipTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified:
		// already passed syntactic verification
		return nil
	case tx.Supernet == constants.PrimaryNetworkID:
		return ErrTransferPermissionlessSupernet
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := verify.All(tx.SupernetAuth, tx.Owner); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *TransferSupernetOwnershipTx) Visit(visitor Visitor) error {
	return visitor.TransferSupernetOwnershipTx(tx)
}
