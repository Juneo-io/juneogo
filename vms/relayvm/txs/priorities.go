// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

const (
	// First primary network apricot delegators are moved from the pending to
	// the current validator set,
	PrimaryNetworkDelegatorApricotPendingPriority Priority = iota + 1
	// then primary network validators,
	PrimaryNetworkValidatorPendingPriority
	// then primary network banff delegators,
	PrimaryNetworkDelegatorBanffPendingPriority
	// then permissionless supernet validators,
	SupernetPermissionlessValidatorPendingPriority
	// then permissionless supernet delegators.
	SupernetPermissionlessDelegatorPendingPriority
	// then permissioned supernet validators,
	SupernetPermissionedValidatorPendingPriority

	// First permissioned supernet validators are removed from the current
	// validator set,
	// Invariant: All permissioned stakers must be removed first because they
	//            are removed by the advancement of time. Permissionless stakers
	//            are removed with a RewardValidatorTx after time has advanced.
	SupernetPermissionedValidatorCurrentPriority
	// then permissionless supernet delegators,
	SupernetPermissionlessDelegatorCurrentPriority
	// then permissionless supernet validators,
	SupernetPermissionlessValidatorCurrentPriority
	// then primary network delegators,
	PrimaryNetworkDelegatorCurrentPriority
	// then primary network validators.
	PrimaryNetworkValidatorCurrentPriority
)

var PendingToCurrentPriorities = []Priority{
	PrimaryNetworkDelegatorApricotPendingPriority:  PrimaryNetworkDelegatorCurrentPriority,
	PrimaryNetworkValidatorPendingPriority:         PrimaryNetworkValidatorCurrentPriority,
	PrimaryNetworkDelegatorBanffPendingPriority:    PrimaryNetworkDelegatorCurrentPriority,
	SupernetPermissionlessValidatorPendingPriority: SupernetPermissionlessValidatorCurrentPriority,
	SupernetPermissionlessDelegatorPendingPriority: SupernetPermissionlessDelegatorCurrentPriority,
	SupernetPermissionedValidatorPendingPriority:   SupernetPermissionedValidatorCurrentPriority,
}

type Priority byte
