// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/google/btree"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
)

type Stakers interface {
	CurrentStakers
	PendingStakers
}

type CurrentStakers interface {
	// GetCurrentValidator returns the [staker] describing the validator on
	// [supernetID] with [nodeID]. If the validator does not exist,
	// [database.ErrNotFound] is returned.
	GetCurrentValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error)

	// PutCurrentValidator adds the [staker] describing a validator to the
	// staker set.
	//
	// Invariant: [staker] is not currently a CurrentValidator
	PutCurrentValidator(staker *Staker)

	// DeleteCurrentValidator removes the [staker] describing a validator from
	// the staker set.
	//
	// Invariant: [staker] is currently a CurrentValidator
	DeleteCurrentValidator(staker *Staker)

	// SetDelegateeReward sets the accrued delegation rewards for [nodeID] on
	// [supernetID] to [amount].
	SetDelegateeReward(supernetID ids.ID, nodeID ids.NodeID, amount uint64) error

	// GetDelegateeReward returns the accrued delegation rewards for [nodeID] on
	// [supernetID].
	GetDelegateeReward(supernetID ids.ID, nodeID ids.NodeID) (uint64, error)

	// GetCurrentDelegatorIterator returns the delegators associated with the
	// validator on [supernetID] with [nodeID]. Delegators are sorted by their
	// removal from current staker set.
	GetCurrentDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error)

	// PutCurrentDelegator adds the [staker] describing a delegator to the
	// staker set.
	//
	// Invariant: [staker] is not currently a CurrentDelegator
	PutCurrentDelegator(staker *Staker)

	// DeleteCurrentDelegator removes the [staker] describing a delegator from
	// the staker set.
	//
	// Invariant: [staker] is currently a CurrentDelegator
	DeleteCurrentDelegator(staker *Staker)

	// GetCurrentStakerIterator returns stakers in order of their removal from
	// the current staker set.
	GetCurrentStakerIterator() (StakerIterator, error)
}

type PendingStakers interface {
	// GetPendingValidator returns the Staker describing the validator on
	// [supernetID] with [nodeID]. If the validator does not exist,
	// [database.ErrNotFound] is returned.
	GetPendingValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error)

	// PutPendingValidator adds the [staker] describing a validator to the
	// staker set.
	PutPendingValidator(staker *Staker)

	// DeletePendingValidator removes the [staker] describing a validator from
	// the staker set.
	DeletePendingValidator(staker *Staker)

	// GetPendingDelegatorIterator returns the delegators associated with the
	// validator on [supernetID] with [nodeID]. Delegators are sorted by their
	// removal from pending staker set.
	GetPendingDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error)

	// PutPendingDelegator adds the [staker] describing a delegator to the
	// staker set.
	PutPendingDelegator(staker *Staker)

	// DeletePendingDelegator removes the [staker] describing a delegator from
	// the staker set.
	DeletePendingDelegator(staker *Staker)

	// GetPendingStakerIterator returns stakers in order of their removal from
	// the pending staker set.
	GetPendingStakerIterator() (StakerIterator, error)
}

type baseStakers struct {
	// supernetID --> nodeID --> current state for the validator of the supernet
	validators map[ids.ID]map[ids.NodeID]*baseStaker
	stakers    *btree.BTreeG[*Staker]
	// supernetID --> nodeID --> diff for that validator since the last db write
	validatorDiffs map[ids.ID]map[ids.NodeID]*diffValidator
}

type baseStaker struct {
	validator  *Staker
	delegators *btree.BTreeG[*Staker]
}

func newBaseStakers() *baseStakers {
	return &baseStakers{
		validators:     make(map[ids.ID]map[ids.NodeID]*baseStaker),
		stakers:        btree.NewG(defaultTreeDegree, (*Staker).Less),
		validatorDiffs: make(map[ids.ID]map[ids.NodeID]*diffValidator),
	}
}

func (v *baseStakers) GetValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error) {
	supernetValidators, ok := v.validators[supernetID]
	if !ok {
		return nil, database.ErrNotFound
	}
	validator, ok := supernetValidators[nodeID]
	if !ok {
		return nil, database.ErrNotFound
	}
	if validator.validator == nil {
		return nil, database.ErrNotFound
	}
	return validator.validator, nil
}

func (v *baseStakers) PutValidator(staker *Staker) {
	validator := v.getOrCreateValidator(staker.SupernetID, staker.NodeID)
	validator.validator = staker

	validatorDiff := v.getOrCreateValidatorDiff(staker.SupernetID, staker.NodeID)
	validatorDiff.validatorStatus = added
	validatorDiff.validator = staker

	v.stakers.ReplaceOrInsert(staker)
}

func (v *baseStakers) DeleteValidator(staker *Staker) {
	validator := v.getOrCreateValidator(staker.SupernetID, staker.NodeID)
	validator.validator = nil
	v.pruneValidator(staker.SupernetID, staker.NodeID)

	validatorDiff := v.getOrCreateValidatorDiff(staker.SupernetID, staker.NodeID)
	validatorDiff.validatorStatus = deleted
	validatorDiff.validator = staker

	v.stakers.Delete(staker)
}

func (v *baseStakers) GetDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) StakerIterator {
	supernetValidators, ok := v.validators[supernetID]
	if !ok {
		return EmptyIterator
	}
	validator, ok := supernetValidators[nodeID]
	if !ok {
		return EmptyIterator
	}
	return NewTreeIterator(validator.delegators)
}

func (v *baseStakers) PutDelegator(staker *Staker) {
	validator := v.getOrCreateValidator(staker.SupernetID, staker.NodeID)
	if validator.delegators == nil {
		validator.delegators = btree.NewG(defaultTreeDegree, (*Staker).Less)
	}
	validator.delegators.ReplaceOrInsert(staker)

	validatorDiff := v.getOrCreateValidatorDiff(staker.SupernetID, staker.NodeID)
	if validatorDiff.addedDelegators == nil {
		validatorDiff.addedDelegators = btree.NewG(defaultTreeDegree, (*Staker).Less)
	}
	validatorDiff.addedDelegators.ReplaceOrInsert(staker)

	v.stakers.ReplaceOrInsert(staker)
}

func (v *baseStakers) DeleteDelegator(staker *Staker) {
	validator := v.getOrCreateValidator(staker.SupernetID, staker.NodeID)
	if validator.delegators != nil {
		validator.delegators.Delete(staker)
	}
	v.pruneValidator(staker.SupernetID, staker.NodeID)

	validatorDiff := v.getOrCreateValidatorDiff(staker.SupernetID, staker.NodeID)
	if validatorDiff.deletedDelegators == nil {
		validatorDiff.deletedDelegators = make(map[ids.ID]*Staker)
	}
	validatorDiff.deletedDelegators[staker.TxID] = staker

	v.stakers.Delete(staker)
}

func (v *baseStakers) GetStakerIterator() StakerIterator {
	return NewTreeIterator(v.stakers)
}

func (v *baseStakers) getOrCreateValidator(supernetID ids.ID, nodeID ids.NodeID) *baseStaker {
	supernetValidators, ok := v.validators[supernetID]
	if !ok {
		supernetValidators = make(map[ids.NodeID]*baseStaker)
		v.validators[supernetID] = supernetValidators
	}
	validator, ok := supernetValidators[nodeID]
	if !ok {
		validator = &baseStaker{}
		supernetValidators[nodeID] = validator
	}
	return validator
}

// pruneValidator assumes that the named validator is currently in the
// [validators] map.
func (v *baseStakers) pruneValidator(supernetID ids.ID, nodeID ids.NodeID) {
	supernetValidators := v.validators[supernetID]
	validator := supernetValidators[nodeID]
	if validator.validator != nil {
		return
	}
	if validator.delegators != nil && validator.delegators.Len() > 0 {
		return
	}
	delete(supernetValidators, nodeID)
	if len(supernetValidators) == 0 {
		delete(v.validators, supernetID)
	}
}

func (v *baseStakers) getOrCreateValidatorDiff(supernetID ids.ID, nodeID ids.NodeID) *diffValidator {
	supernetValidatorDiffs, ok := v.validatorDiffs[supernetID]
	if !ok {
		supernetValidatorDiffs = make(map[ids.NodeID]*diffValidator)
		v.validatorDiffs[supernetID] = supernetValidatorDiffs
	}
	validatorDiff, ok := supernetValidatorDiffs[nodeID]
	if !ok {
		validatorDiff = &diffValidator{
			validatorStatus: unmodified,
		}
		supernetValidatorDiffs[nodeID] = validatorDiff
	}
	return validatorDiff
}

type diffStakers struct {
	// supernetID --> nodeID --> diff for that validator
	validatorDiffs map[ids.ID]map[ids.NodeID]*diffValidator
	addedStakers   *btree.BTreeG[*Staker]
	deletedStakers map[ids.ID]*Staker
}

type diffValidator struct {
	// validatorStatus describes whether a validator has been added or removed.
	//
	// validatorStatus is not affected by delegators ops so unmodified does not
	// mean that diffValidator hasn't change, since delegators may have changed.
	validatorStatus diffValidatorStatus
	validator       *Staker

	addedDelegators   *btree.BTreeG[*Staker]
	deletedDelegators map[ids.ID]*Staker
}

// GetValidator attempts to fetch the validator with the given supernetID and
// nodeID.
// Invariant: Assumes that the validator will never be removed and then added.
func (s *diffStakers) GetValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, diffValidatorStatus) {
	supernetValidatorDiffs, ok := s.validatorDiffs[supernetID]
	if !ok {
		return nil, unmodified
	}

	validatorDiff, ok := supernetValidatorDiffs[nodeID]
	if !ok {
		return nil, unmodified
	}

	if validatorDiff.validatorStatus == added {
		return validatorDiff.validator, added
	}
	return nil, validatorDiff.validatorStatus
}

func (s *diffStakers) PutValidator(staker *Staker) {
	validatorDiff := s.getOrCreateDiff(staker.SupernetID, staker.NodeID)
	validatorDiff.validatorStatus = added
	validatorDiff.validator = staker

	if s.addedStakers == nil {
		s.addedStakers = btree.NewG(defaultTreeDegree, (*Staker).Less)
	}
	s.addedStakers.ReplaceOrInsert(staker)
}

func (s *diffStakers) DeleteValidator(staker *Staker) {
	validatorDiff := s.getOrCreateDiff(staker.SupernetID, staker.NodeID)
	if validatorDiff.validatorStatus == added {
		// This validator was added and immediately removed in this diff. We
		// treat it as if it was never added.
		validatorDiff.validatorStatus = unmodified
		s.addedStakers.Delete(validatorDiff.validator)
		validatorDiff.validator = nil
	} else {
		validatorDiff.validatorStatus = deleted
		validatorDiff.validator = staker
		if s.deletedStakers == nil {
			s.deletedStakers = make(map[ids.ID]*Staker)
		}
		s.deletedStakers[staker.TxID] = staker
	}
}

func (s *diffStakers) GetDelegatorIterator(
	parentIterator StakerIterator,
	supernetID ids.ID,
	nodeID ids.NodeID,
) StakerIterator {
	var (
		addedDelegatorIterator = EmptyIterator
		deletedDelegators      map[ids.ID]*Staker
	)
	if supernetValidatorDiffs, ok := s.validatorDiffs[supernetID]; ok {
		if validatorDiff, ok := supernetValidatorDiffs[nodeID]; ok {
			addedDelegatorIterator = NewTreeIterator(validatorDiff.addedDelegators)
			deletedDelegators = validatorDiff.deletedDelegators
		}
	}

	return NewMaskedIterator(
		NewMergedIterator(
			parentIterator,
			addedDelegatorIterator,
		),
		deletedDelegators,
	)
}

func (s *diffStakers) PutDelegator(staker *Staker) {
	validatorDiff := s.getOrCreateDiff(staker.SupernetID, staker.NodeID)
	if validatorDiff.addedDelegators == nil {
		validatorDiff.addedDelegators = btree.NewG(defaultTreeDegree, (*Staker).Less)
	}
	validatorDiff.addedDelegators.ReplaceOrInsert(staker)

	if s.addedStakers == nil {
		s.addedStakers = btree.NewG(defaultTreeDegree, (*Staker).Less)
	}
	s.addedStakers.ReplaceOrInsert(staker)
}

func (s *diffStakers) DeleteDelegator(staker *Staker) {
	validatorDiff := s.getOrCreateDiff(staker.SupernetID, staker.NodeID)
	if validatorDiff.deletedDelegators == nil {
		validatorDiff.deletedDelegators = make(map[ids.ID]*Staker)
	}
	validatorDiff.deletedDelegators[staker.TxID] = staker

	if s.deletedStakers == nil {
		s.deletedStakers = make(map[ids.ID]*Staker)
	}
	s.deletedStakers[staker.TxID] = staker
}

func (s *diffStakers) GetStakerIterator(parentIterator StakerIterator) StakerIterator {
	return NewMaskedIterator(
		NewMergedIterator(
			parentIterator,
			NewTreeIterator(s.addedStakers),
		),
		s.deletedStakers,
	)
}

func (s *diffStakers) getOrCreateDiff(supernetID ids.ID, nodeID ids.NodeID) *diffValidator {
	if s.validatorDiffs == nil {
		s.validatorDiffs = make(map[ids.ID]map[ids.NodeID]*diffValidator)
	}
	supernetValidatorDiffs, ok := s.validatorDiffs[supernetID]
	if !ok {
		supernetValidatorDiffs = make(map[ids.NodeID]*diffValidator)
		s.validatorDiffs[supernetID] = supernetValidatorDiffs
	}
	validatorDiff, ok := supernetValidatorDiffs[nodeID]
	if !ok {
		validatorDiff = &diffValidator{
			validatorStatus: unmodified,
		}
		supernetValidatorDiffs[nodeID] = validatorDiff
	}
	return validatorDiff
}
