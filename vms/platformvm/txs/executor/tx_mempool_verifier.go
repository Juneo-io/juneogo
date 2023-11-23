// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var _ txs.Visitor = (*MempoolTxVerifier)(nil)

type MempoolTxVerifier struct {
	*Backend
	ParentID      ids.ID
	StateVersions state.Versions
	Tx            *txs.Tx
}

func (*MempoolTxVerifier) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	return ErrWrongTxType
}

func (*MempoolTxVerifier) RewardValidatorTx(*txs.RewardValidatorTx) error {
	return ErrWrongTxType
}

func (v *MempoolTxVerifier) AddValidatorTx(tx *txs.AddValidatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) AddSupernetValidatorTx(tx *txs.AddSupernetValidatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) AddDelegatorTx(tx *txs.AddDelegatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) CreateChainTx(tx *txs.CreateChainTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) CreateSupernetTx(tx *txs.CreateSupernetTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) ImportTx(tx *txs.ImportTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) ExportTx(tx *txs.ExportTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) RemoveSupernetValidatorTx(tx *txs.RemoveSupernetValidatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) TransformSupernetTx(tx *txs.TransformSupernetTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) AddPermissionlessValidatorTx(tx *txs.AddPermissionlessValidatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) AddPermissionlessDelegatorTx(tx *txs.AddPermissionlessDelegatorTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) TransferSupernetOwnershipTx(tx *txs.TransferSupernetOwnershipTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) standardTx(tx txs.UnsignedTx) error {
	baseState, err := v.standardBaseState()
	if err != nil {
		return err
	}

	executor := StandardTxExecutor{
		Backend: v.Backend,
		State:   baseState,
		Tx:      v.Tx,
	}
	err = tx.Visit(&executor)
	// We ignore [errFutureStakeTime] here because the time will be advanced
	// when this transaction is issued.
	if errors.Is(err, ErrFutureStakeTime) {
		return nil
	}
	return err
}

// Upon Banff activation, txs are not verified against current chain time
// but against the block timestamp. [baseTime] calculates
// the right timestamp to be used to mempool tx verification
func (v *MempoolTxVerifier) standardBaseState() (state.Diff, error) {
	state, err := state.NewDiff(v.ParentID, v.StateVersions)
	if err != nil {
		return nil, err
	}

	nextBlkTime, err := v.nextBlockTime(state)
	if err != nil {
		return nil, err
	}

	if !v.Backend.Config.IsBanffActivated(nextBlkTime) {
		// next tx would be included into an Apricot block
		// so we verify it against current chain state
		return state, nil
	}

	// next tx would be included into a Banff block
	// so we verify it against duly updated chain state
	changes, err := AdvanceTimeTo(v.Backend, state, nextBlkTime)
	if err != nil {
		return nil, err
	}
	changes.Apply(state)
	state.SetTimestamp(nextBlkTime)

	return state, nil
}

func (v *MempoolTxVerifier) nextBlockTime(state state.Diff) (time.Time, error) {
	var (
		parentTime  = state.GetTimestamp()
		nextBlkTime = v.Clk.Time()
	)
	if parentTime.After(nextBlkTime) {
		nextBlkTime = parentTime
	}
	nextStakerChangeTime, err := GetNextStakerChangeTime(state)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not calculate next staker change time: %w", err)
	}
	if !nextBlkTime.Before(nextStakerChangeTime) {
		nextBlkTime = nextStakerChangeTime
	}
	return nextBlkTime, nil
}
