// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import "github.com/Juneo-io/juneogo/vms/platformvm/txs"

var _ txs.Visitor = (*remover)(nil)

type remover struct {
	m  *mempool
	tx *txs.Tx
}

func (r *remover) AddValidatorTx(*txs.AddValidatorTx) error {
	r.m.removeStakerTx(r.tx)
	return nil
}

func (r *remover) AddSupernetValidatorTx(*txs.AddSupernetValidatorTx) error {
	r.m.removeStakerTx(r.tx)
	return nil
}

func (r *remover) AddDelegatorTx(*txs.AddDelegatorTx) error {
	r.m.removeStakerTx(r.tx)
	return nil
}

func (r *remover) RemoveSupernetValidatorTx(*txs.RemoveSupernetValidatorTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) CreateChainTx(*txs.CreateChainTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) CreateSupernetTx(*txs.CreateSupernetTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) ImportTx(*txs.ImportTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) ExportTx(*txs.ExportTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) TransformSupernetTx(*txs.TransformSupernetTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) TransferSupernetOwnershipTx(*txs.TransferSupernetOwnershipTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}

func (r *remover) AddPermissionlessValidatorTx(*txs.AddPermissionlessValidatorTx) error {
	r.m.removeStakerTx(r.tx)
	return nil
}

func (r *remover) AddPermissionlessDelegatorTx(*txs.AddPermissionlessDelegatorTx) error {
	r.m.removeStakerTx(r.tx)
	return nil
}

func (*remover) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	// this tx is never in mempool
	return nil
}

func (*remover) RewardValidatorTx(*txs.RewardValidatorTx) error {
	// this tx is never in mempool
	return nil
}
