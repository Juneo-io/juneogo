// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var _ txs.Visitor = (*AtomicTxExecutor)(nil)

// atomicTxExecutor is used to execute atomic transactions pre-AP5. After AP5
// the execution was moved to be performed inside of the standardTxExecutor.
type AtomicTxExecutor struct {
	// inputs, to be filled before visitor methods are called
	*Backend
	ParentID      ids.ID
	StateVersions state.Versions
	Tx            *txs.Tx

	// outputs of visitor execution
	OnAccept       state.Diff
	Inputs         set.Set[ids.ID]
	AtomicRequests map[ids.ID]*atomic.Requests
}

func (*AtomicTxExecutor) AddValidatorTx(*txs.AddValidatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddSupernetValidatorTx(*txs.AddSupernetValidatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddDelegatorTx(*txs.AddDelegatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) CreateChainTx(*txs.CreateChainTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) CreateSupernetTx(*txs.CreateSupernetTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) RewardValidatorTx(*txs.RewardValidatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) RemoveSupernetValidatorTx(*txs.RemoveSupernetValidatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) TransformSupernetTx(*txs.TransformSupernetTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) TransferSupernetOwnershipTx(*txs.TransferSupernetOwnershipTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddPermissionlessValidatorTx(*txs.AddPermissionlessValidatorTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddPermissionlessDelegatorTx(*txs.AddPermissionlessDelegatorTx) error {
	return ErrWrongTxType
}

func (e *AtomicTxExecutor) ImportTx(tx *txs.ImportTx) error {
	return e.atomicTx(tx)
}

func (e *AtomicTxExecutor) ExportTx(tx *txs.ExportTx) error {
	return e.atomicTx(tx)
}

func (e *AtomicTxExecutor) atomicTx(tx txs.UnsignedTx) error {
	onAccept, err := state.NewDiff(
		e.ParentID,
		e.StateVersions,
	)
	if err != nil {
		return err
	}
	e.OnAccept = onAccept

	executor := StandardTxExecutor{
		Backend: e.Backend,
		State:   e.OnAccept,
		Tx:      e.Tx,
	}
	err = tx.Visit(&executor)
	e.Inputs = executor.Inputs
	e.AtomicRequests = executor.AtomicRequests
	return err
}
