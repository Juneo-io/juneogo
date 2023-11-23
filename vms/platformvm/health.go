// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/utils/constants"
)

func (vm *VM) HealthCheck(context.Context) (interface{}, error) {
	localPrimaryValidator, err := vm.state.GetCurrentValidator(
		constants.PrimaryNetworkID,
		vm.ctx.NodeID,
	)
	switch err {
	case nil:
		vm.metrics.SetTimeUntilUnstake(time.Until(localPrimaryValidator.EndTime))
	case database.ErrNotFound:
		vm.metrics.SetTimeUntilUnstake(0)
	default:
		return nil, fmt.Errorf("couldn't get current local validator: %w", err)
	}

	for supernetID := range vm.TrackedSupernets {
		localSupernetValidator, err := vm.state.GetCurrentValidator(
			supernetID,
			vm.ctx.NodeID,
		)
		switch err {
		case nil:
			vm.metrics.SetTimeUntilSupernetUnstake(supernetID, time.Until(localSupernetValidator.EndTime))
		case database.ErrNotFound:
			vm.metrics.SetTimeUntilSupernetUnstake(supernetID, 0)
		default:
			return nil, fmt.Errorf("couldn't get current supernet validator of %q: %w", supernetID, err)
		}
	}
	return nil, nil
}
