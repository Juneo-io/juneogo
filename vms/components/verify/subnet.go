// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package verify

import (
	"context"
	"errors"
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
)

var (
	errSameChainID           = errors.New("same chainID")
	errMismatchedSupernetIDs = errors.New("mismatched supernetIDs")
)

// SameSupernet verifies that the provided [ctx] was provided to a chain in the
// same supernet as [peerChainID], but not the same chain. If this verification
// fails, a non-nil error will be returned.
func SameSupernet(ctx context.Context, chainCtx *snow.Context, peerChainID ids.ID) error {
	if peerChainID == chainCtx.ChainID {
		return errSameChainID
	}

	supernetID, err := chainCtx.ValidatorState.GetSupernetID(ctx, peerChainID)
	if err != nil {
		return fmt.Errorf("failed to get supernet of %q: %w", peerChainID, err)
	}
	if chainCtx.SupernetID != supernetID {
		return fmt.Errorf("%w; expected %q got %q", errMismatchedSupernetIDs, chainCtx.SupernetID, supernetID)
	}
	return nil
}
