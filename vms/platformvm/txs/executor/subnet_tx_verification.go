// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var (
	errWrongNumberOfCredentials       = errors.New("should have the same number of credentials as inputs")
	errIsImmutable                    = errors.New("is immutable")
	errUnauthorizedSupernetModification = errors.New("unauthorized supernet modification")
)

// verifyPoASupernetAuthorization carries out the validation for modifying a PoA
// supernet. This is an extension of [verifySupernetAuthorization] that additionally
// verifies that the supernet being modified is currently a PoA supernet.
func verifyPoASupernetAuthorization(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	supernetID ids.ID,
	supernetAuth verify.Verifiable,
) ([]verify.Verifiable, error) {
	creds, err := verifySupernetAuthorization(backend, chainState, sTx, supernetID, supernetAuth)
	if err != nil {
		return nil, err
	}

	_, err = chainState.GetSupernetTransformation(supernetID)
	if err == nil {
		return nil, fmt.Errorf("%q %w", supernetID, errIsImmutable)
	}
	if err != database.ErrNotFound {
		return nil, err
	}

	return creds, nil
}

// verifySupernetAuthorization carries out the validation for modifying a supernet.
// The last credential in [sTx.Creds] is used as the supernet authorization.
// Returns the remaining tx credentials that should be used to authorize the
// other operations in the tx.
func verifySupernetAuthorization(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	supernetID ids.ID,
	supernetAuth verify.Verifiable,
) ([]verify.Verifiable, error) {
	if len(sTx.Creds) == 0 {
		// Ensure there is at least one credential for the supernet authorization
		return nil, errWrongNumberOfCredentials
	}

	baseTxCredsLen := len(sTx.Creds) - 1
	supernetCred := sTx.Creds[baseTxCredsLen]

	supernetOwner, err := chainState.GetSupernetOwner(supernetID)
	if err != nil {
		return nil, err
	}

	if err := backend.Fx.VerifyPermission(sTx.Unsigned, supernetAuth, supernetCred, supernetOwner); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnauthorizedSupernetModification, err)
	}

	return sTx.Creds[:baseTxCredsLen], nil
}
