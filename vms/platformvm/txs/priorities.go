// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
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
	PrimaryNetworkDelegatorApricotPendingPriority: PrimaryNetworkDelegatorCurrentPriority,
	PrimaryNetworkValidatorPendingPriority:        PrimaryNetworkValidatorCurrentPriority,
	PrimaryNetworkDelegatorBanffPendingPriority:   PrimaryNetworkDelegatorCurrentPriority,
	SupernetPermissionlessValidatorPendingPriority:  SupernetPermissionlessValidatorCurrentPriority,
	SupernetPermissionlessDelegatorPendingPriority:  SupernetPermissionlessDelegatorCurrentPriority,
	SupernetPermissionedValidatorPendingPriority:    SupernetPermissionedValidatorCurrentPriority,
}

type Priority byte

func (p Priority) IsCurrent() bool {
	return p.IsCurrentValidator() || p.IsCurrentDelegator()
}

func (p Priority) IsPending() bool {
	return p.IsPendingValidator() || p.IsPendingDelegator()
}

func (p Priority) IsValidator() bool {
	return p.IsCurrentValidator() || p.IsPendingValidator()
}

func (p Priority) IsPermissionedValidator() bool {
	return p == SupernetPermissionedValidatorCurrentPriority ||
		p == SupernetPermissionedValidatorPendingPriority
}

func (p Priority) IsDelegator() bool {
	return p.IsCurrentDelegator() || p.IsPendingDelegator()
}

func (p Priority) IsCurrentValidator() bool {
	return p == PrimaryNetworkValidatorCurrentPriority ||
		p == SupernetPermissionedValidatorCurrentPriority ||
		p == SupernetPermissionlessValidatorCurrentPriority
}

func (p Priority) IsCurrentDelegator() bool {
	return p == PrimaryNetworkDelegatorCurrentPriority ||
		p == SupernetPermissionlessDelegatorCurrentPriority
}

func (p Priority) IsPendingValidator() bool {
	return p == PrimaryNetworkValidatorPendingPriority ||
		p == SupernetPermissionedValidatorPendingPriority ||
		p == SupernetPermissionlessValidatorPendingPriority
}

func (p Priority) IsPendingDelegator() bool {
	return p == PrimaryNetworkDelegatorBanffPendingPriority ||
		p == PrimaryNetworkDelegatorApricotPendingPriority ||
		p == SupernetPermissionlessDelegatorPendingPriority
}
