// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/status"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

var (
	_ Diff = (*diff)(nil)

	ErrMissingParentState = errors.New("missing parent state")
)

type Diff interface {
	Chain

	Apply(State)
}

type diff struct {
	parentID      ids.ID
	stateVersions Versions

	timestamp time.Time

	// Supernet ID --> supply of native asset of the supernet
	currentSupply map[ids.ID]uint64
	// Supernet ID --> supply of rewards pool of the supernet
	rewardsPoolSupply map[ids.ID]uint64
	feesPoolValue     uint64

	currentStakerDiffs diffStakers
	pendingStakerDiffs diffStakers

	addedSupernets []*txs.Tx
	// Supernet ID --> Tx that transforms the supernet
	transformedSupernets map[ids.ID]*txs.Tx
	cachedSupernets      []*txs.Tx

	addedChains  map[ids.ID][]*txs.Tx
	cachedChains map[ids.ID][]*txs.Tx

	// map of txID -> []*UTXO
	addedRewardUTXOs map[ids.ID][]*june.UTXO

	// map of txID -> {*txs.Tx, Status}
	addedTxs map[ids.ID]*txAndStatus

	// map of modified UTXOID -> *UTXO if the UTXO is nil, it has been removed
	modifiedUTXOs map[ids.ID]*utxoModification
}

type utxoModification struct {
	utxoID ids.ID
	utxo   *june.UTXO
}

func NewDiff(
	parentID ids.ID,
	stateVersions Versions,
) (Diff, error) {
	parentState, ok := stateVersions.GetState(parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, parentID)
	}
	return &diff{
		parentID:      parentID,
		stateVersions: stateVersions,
		timestamp:     parentState.GetTimestamp(),
	}, nil
}

func (d *diff) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *diff) SetTimestamp(timestamp time.Time) {
	d.timestamp = timestamp
}

func (d *diff) GetCurrentSupply(supernetID ids.ID) (uint64, error) {
	supply, ok := d.currentSupply[supernetID]
	if ok {
		return supply, nil
	}

	// If the supernet supply wasn't modified in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetCurrentSupply(supernetID)
}

func (d *diff) SetCurrentSupply(supernetID ids.ID, currentSupply uint64) {
	if d.currentSupply == nil {
		d.currentSupply = map[ids.ID]uint64{
			supernetID: currentSupply,
		}
	} else {
		d.currentSupply[supernetID] = currentSupply
	}
}

func (d *diff) GetRewardsPoolSupply(supernetID ids.ID) (uint64, error) {
	rewardsSupply, ok := d.rewardsPoolSupply[supernetID]
	if ok {
		return rewardsSupply, nil
	}

	// If the supernet supply wasn't modified in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetRewardsPoolSupply(supernetID)
}

func (d *diff) SetRewardsPoolSupply(supernetID ids.ID, rewardsPoolSupply uint64) {
	if d.rewardsPoolSupply == nil {
		d.rewardsPoolSupply = map[ids.ID]uint64{
			supernetID: rewardsPoolSupply,
		}
	} else {
		d.rewardsPoolSupply[supernetID] = rewardsPoolSupply
	}
}

func (d *diff) GetFeesPoolValue() uint64 {
	return d.feesPoolValue
}

func (d *diff) SetFeesPoolValue(feesPoolValue uint64) {
	d.feesPoolValue = feesPoolValue
}

func (d *diff) GetCurrentValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error) {
	// If the validator was modified in this diff, return the modified
	// validator.
	newValidator, ok := d.currentStakerDiffs.GetValidator(supernetID, nodeID)
	if ok {
		if newValidator == nil {
			return nil, database.ErrNotFound
		}
		return newValidator, nil
	}

	// If the validator wasn't modified in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetCurrentValidator(supernetID, nodeID)
}

func (d *diff) PutCurrentValidator(staker *Staker) {
	d.currentStakerDiffs.PutValidator(staker)
}

func (d *diff) DeleteCurrentValidator(staker *Staker) {
	d.currentStakerDiffs.DeleteValidator(staker)
}

func (d *diff) GetCurrentDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentIterator, err := parentState.GetCurrentDelegatorIterator(supernetID, nodeID)
	if err != nil {
		return nil, err
	}

	return d.currentStakerDiffs.GetDelegatorIterator(parentIterator, supernetID, nodeID), nil
}

func (d *diff) PutCurrentDelegator(staker *Staker) {
	d.currentStakerDiffs.PutDelegator(staker)
}

func (d *diff) DeleteCurrentDelegator(staker *Staker) {
	d.currentStakerDiffs.DeleteDelegator(staker)
}

func (d *diff) GetCurrentStakerIterator() (StakerIterator, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentIterator, err := parentState.GetCurrentStakerIterator()
	if err != nil {
		return nil, err
	}

	return d.currentStakerDiffs.GetStakerIterator(parentIterator), nil
}

func (d *diff) GetPendingValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error) {
	// If the validator was modified in this diff, return the modified
	// validator.
	newValidator, ok := d.pendingStakerDiffs.GetValidator(supernetID, nodeID)
	if ok {
		if newValidator == nil {
			return nil, database.ErrNotFound
		}
		return newValidator, nil
	}

	// If the validator wasn't modified in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetPendingValidator(supernetID, nodeID)
}

func (d *diff) PutPendingValidator(staker *Staker) {
	d.pendingStakerDiffs.PutValidator(staker)
}

func (d *diff) DeletePendingValidator(staker *Staker) {
	d.pendingStakerDiffs.DeleteValidator(staker)
}

func (d *diff) GetPendingDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentIterator, err := parentState.GetPendingDelegatorIterator(supernetID, nodeID)
	if err != nil {
		return nil, err
	}

	return d.pendingStakerDiffs.GetDelegatorIterator(parentIterator, supernetID, nodeID), nil
}

func (d *diff) PutPendingDelegator(staker *Staker) {
	d.pendingStakerDiffs.PutDelegator(staker)
}

func (d *diff) DeletePendingDelegator(staker *Staker) {
	d.pendingStakerDiffs.DeleteDelegator(staker)
}

func (d *diff) GetPendingStakerIterator() (StakerIterator, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentIterator, err := parentState.GetPendingStakerIterator()
	if err != nil {
		return nil, err
	}

	return d.pendingStakerDiffs.GetStakerIterator(parentIterator), nil
}

func (d *diff) GetSupernets() ([]*txs.Tx, error) {
	if len(d.addedSupernets) == 0 {
		parentState, ok := d.stateVersions.GetState(d.parentID)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
		}
		return parentState.GetSupernets()
	}

	if len(d.cachedSupernets) != 0 {
		return d.cachedSupernets, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	supernets, err := parentState.GetSupernets()
	if err != nil {
		return nil, err
	}
	newSupernets := make([]*txs.Tx, len(supernets)+len(d.addedSupernets))
	copy(newSupernets, supernets)
	for i, supernet := range d.addedSupernets {
		newSupernets[i+len(supernets)] = supernet
	}
	d.cachedSupernets = newSupernets
	return newSupernets, nil
}

func (d *diff) AddSupernet(createSupernetTx *txs.Tx) {
	d.addedSupernets = append(d.addedSupernets, createSupernetTx)
	if d.cachedSupernets != nil {
		d.cachedSupernets = append(d.cachedSupernets, createSupernetTx)
	}
}

func (d *diff) GetSupernetTransformation(supernetID ids.ID) (*txs.Tx, error) {
	tx, exists := d.transformedSupernets[supernetID]
	if exists {
		return tx, nil
	}

	// If the supernet wasn't transformed in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, ErrMissingParentState
	}
	return parentState.GetSupernetTransformation(supernetID)
}

func (d *diff) AddSupernetTransformation(transformSupernetTxIntf *txs.Tx) {
	transformSupernetTx := transformSupernetTxIntf.Unsigned.(*txs.TransformSupernetTx)
	if d.transformedSupernets == nil {
		d.transformedSupernets = map[ids.ID]*txs.Tx{
			transformSupernetTx.Supernet: transformSupernetTxIntf,
		}
	} else {
		d.transformedSupernets[transformSupernetTx.Supernet] = transformSupernetTxIntf
	}
}

func (d *diff) GetChains(supernetID ids.ID) ([]*txs.Tx, error) {
	addedChains := d.addedChains[supernetID]
	if len(addedChains) == 0 {
		// No chains have been added to this supernet
		parentState, ok := d.stateVersions.GetState(d.parentID)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
		}
		return parentState.GetChains(supernetID)
	}

	// There have been chains added to the requested supernet

	if d.cachedChains == nil {
		// This is the first time we are going to be caching the supernet chains
		d.cachedChains = make(map[ids.ID][]*txs.Tx)
	}

	cachedChains, cached := d.cachedChains[supernetID]
	if cached {
		return cachedChains, nil
	}

	// This chain wasn't cached yet
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	chains, err := parentState.GetChains(supernetID)
	if err != nil {
		return nil, err
	}

	newChains := make([]*txs.Tx, len(chains)+len(addedChains))
	copy(newChains, chains)
	for i, chain := range addedChains {
		newChains[i+len(chains)] = chain
	}
	d.cachedChains[supernetID] = newChains
	return newChains, nil
}

func (d *diff) AddChain(createChainTx *txs.Tx) {
	tx := createChainTx.Unsigned.(*txs.CreateChainTx)
	if d.addedChains == nil {
		d.addedChains = map[ids.ID][]*txs.Tx{
			tx.SupernetID: {createChainTx},
		}
	} else {
		d.addedChains[tx.SupernetID] = append(d.addedChains[tx.SupernetID], createChainTx)
	}

	cachedChains, cached := d.cachedChains[tx.SupernetID]
	if !cached {
		return
	}
	d.cachedChains[tx.SupernetID] = append(cachedChains, createChainTx)
}

func (d *diff) GetTx(txID ids.ID) (*txs.Tx, status.Status, error) {
	if tx, exists := d.addedTxs[txID]; exists {
		return tx.tx, tx.status, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, status.Unknown, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetTx(txID)
}

func (d *diff) AddTx(tx *txs.Tx, stat status.Status, feesAssetID ids.ID) {
	txID := tx.ID()
	txStatus := &txAndStatus{
		tx:     tx,
		status: stat,
	}
	if d.addedTxs == nil {
		d.addedTxs = map[ids.ID]*txAndStatus{
			txID: txStatus,
		}
	} else {
		d.addedTxs[txID] = txStatus
	}
	if stat == status.Processing || stat == status.Committed {
		newFeesPoolValue, err := math.Add64(d.GetFeesPoolValue(), tx.Unsigned.ConsumedValue(feesAssetID))
		if err == nil {
			d.SetFeesPoolValue(newFeesPoolValue)
		}
	}
}

func (d *diff) GetRewardUTXOs(txID ids.ID) ([]*june.UTXO, error) {
	if utxos, exists := d.addedRewardUTXOs[txID]; exists {
		return utxos, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetRewardUTXOs(txID)
}

func (d *diff) AddRewardUTXO(txID ids.ID, utxo *june.UTXO) {
	if d.addedRewardUTXOs == nil {
		d.addedRewardUTXOs = make(map[ids.ID][]*june.UTXO)
	}
	d.addedRewardUTXOs[txID] = append(d.addedRewardUTXOs[txID], utxo)
}

func (d *diff) GetUTXO(utxoID ids.ID) (*june.UTXO, error) {
	utxo, modified := d.modifiedUTXOs[utxoID]
	if !modified {
		parentState, ok := d.stateVersions.GetState(d.parentID)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
		}
		return parentState.GetUTXO(utxoID)
	}
	if utxo.utxo == nil {
		return nil, database.ErrNotFound
	}
	return utxo.utxo, nil
}

func (d *diff) AddUTXO(utxo *june.UTXO) {
	newUTXO := &utxoModification{
		utxoID: utxo.InputID(),
		utxo:   utxo,
	}
	if d.modifiedUTXOs == nil {
		d.modifiedUTXOs = map[ids.ID]*utxoModification{
			utxo.InputID(): newUTXO,
		}
	} else {
		d.modifiedUTXOs[utxo.InputID()] = newUTXO
	}
}

func (d *diff) DeleteUTXO(utxoID ids.ID) {
	newUTXO := &utxoModification{
		utxoID: utxoID,
	}
	if d.modifiedUTXOs == nil {
		d.modifiedUTXOs = map[ids.ID]*utxoModification{
			utxoID: newUTXO,
		}
	} else {
		d.modifiedUTXOs[utxoID] = newUTXO
	}
}

func (d *diff) Apply(baseState State) {
	baseState.SetTimestamp(d.timestamp)
	for supernetID, supply := range d.currentSupply {
		baseState.SetCurrentSupply(supernetID, supply)
	}
	for supernetID, rewardsSupply := range d.rewardsPoolSupply {
		baseState.SetRewardsPoolSupply(supernetID, rewardsSupply)
	}
	baseState.SetFeesPoolValue(d.feesPoolValue)
	for _, supernetValidatorDiffs := range d.currentStakerDiffs.validatorDiffs {
		for _, validatorDiff := range supernetValidatorDiffs {
			if validatorDiff.validatorModified {
				if validatorDiff.validatorDeleted {
					baseState.DeleteCurrentValidator(validatorDiff.validator)
				} else {
					baseState.PutCurrentValidator(validatorDiff.validator)
				}
			}

			addedDelegatorIterator := NewTreeIterator(validatorDiff.addedDelegators)
			for addedDelegatorIterator.Next() {
				baseState.PutCurrentDelegator(addedDelegatorIterator.Value())
			}
			addedDelegatorIterator.Release()

			for _, delegator := range validatorDiff.deletedDelegators {
				baseState.DeleteCurrentDelegator(delegator)
			}
		}
	}
	for _, supernetValidatorDiffs := range d.pendingStakerDiffs.validatorDiffs {
		for _, validatorDiff := range supernetValidatorDiffs {
			if validatorDiff.validatorModified {
				if validatorDiff.validatorDeleted {
					baseState.DeletePendingValidator(validatorDiff.validator)
				} else {
					baseState.PutPendingValidator(validatorDiff.validator)
				}
			}

			addedDelegatorIterator := NewTreeIterator(validatorDiff.addedDelegators)
			for addedDelegatorIterator.Next() {
				baseState.PutPendingDelegator(addedDelegatorIterator.Value())
			}
			addedDelegatorIterator.Release()

			for _, delegator := range validatorDiff.deletedDelegators {
				baseState.DeletePendingDelegator(delegator)
			}
		}
	}
	for _, supernet := range d.addedSupernets {
		baseState.AddSupernet(supernet)
	}
	for _, tx := range d.transformedSupernets {
		baseState.AddSupernetTransformation(tx)
	}
	for _, chains := range d.addedChains {
		for _, chain := range chains {
			baseState.AddChain(chain)
		}
	}
	for _, tx := range d.addedTxs {
		baseState.AddTx(tx.tx, tx.status, ids.Empty)
	}
	for txID, utxos := range d.addedRewardUTXOs {
		for _, utxo := range utxos {
			baseState.AddRewardUTXO(txID, utxo)
		}
	}
	for _, utxo := range d.modifiedUTXOs {
		if utxo.utxo != nil {
			baseState.AddUTXO(utxo.utxo)
		} else {
			baseState.DeleteUTXO(utxo.utxoID)
		}
	}
}
