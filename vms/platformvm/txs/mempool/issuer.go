// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"errors"

	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var (
	_ txs.Visitor = (*issuer)(nil)

	errCantIssueAdvanceTimeTx     = errors.New("can not issue an advance time tx")
	errCantIssueRewardValidatorTx = errors.New("can not issue a reward validator tx")
)

type issuer struct {
	m  *mempool
	tx *txs.Tx
}

func (*issuer) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	return errCantIssueAdvanceTimeTx
}

func (*issuer) RewardValidatorTx(*txs.RewardValidatorTx) error {
	return errCantIssueRewardValidatorTx
}

func (i *issuer) AddValidatorTx(*txs.AddValidatorTx) error {
	i.m.addStakerTx(i.tx)
	return nil
}

func (i *issuer) AddSupernetValidatorTx(*txs.AddSupernetValidatorTx) error {
	i.m.addStakerTx(i.tx)
	return nil
}

func (i *issuer) AddDelegatorTx(*txs.AddDelegatorTx) error {
	i.m.addStakerTx(i.tx)
	return nil
}

func (i *issuer) RemoveSupernetValidatorTx(*txs.RemoveSupernetValidatorTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) CreateChainTx(*txs.CreateChainTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) CreateSupernetTx(*txs.CreateSupernetTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) ImportTx(*txs.ImportTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) ExportTx(*txs.ExportTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) TransformSupernetTx(*txs.TransformSupernetTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}

func (i *issuer) AddPermissionlessValidatorTx(*txs.AddPermissionlessValidatorTx) error {
	i.m.addStakerTx(i.tx)
	return nil
}

func (i *issuer) AddPermissionlessDelegatorTx(*txs.AddPermissionlessDelegatorTx) error {
	i.m.addStakerTx(i.tx)
	return nil
}
