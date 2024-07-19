// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

type addValidatorRules struct {
	assetID           ids.ID
	minValidatorStake uint64
	maxValidatorStake uint64
	minStakeDuration  time.Duration
	maxStakeDuration  time.Duration
	minDelegationFee  uint32
	maxDelegationFee  uint32
}

func getValidatorRules(
	backend *Backend,
	chainState state.Chain,
	supernetID ids.ID,
) (*addValidatorRules, error) {
	if supernetID == constants.PrimaryNetworkID {
		return &addValidatorRules{
			assetID:           backend.Ctx.JUNEAssetID,
			minValidatorStake: backend.Config.MinValidatorStake,
			maxValidatorStake: backend.Config.MaxValidatorStake,
			minStakeDuration:  backend.Config.MinStakeDuration,
			maxStakeDuration:  backend.Config.MaxStakeDuration,
			minDelegationFee:  backend.Config.MinDelegationFee,
			maxDelegationFee:  backend.Config.MaxDelegationFee,
		}, nil
	}

	transformSupernet, err := GetTransformSupernetTx(chainState, supernetID)
	if err != nil {
		return nil, err
	}

	return &addValidatorRules{
		assetID:           transformSupernet.AssetID,
		minValidatorStake: transformSupernet.MinValidatorStake,
		maxValidatorStake: transformSupernet.MaxValidatorStake,
		minStakeDuration:  time.Duration(transformSupernet.MinStakeDuration) * time.Second,
		maxStakeDuration:  time.Duration(transformSupernet.MaxStakeDuration) * time.Second,
		minDelegationFee:  transformSupernet.MinDelegationFee,
		maxDelegationFee:  transformSupernet.MaxDelegationFee,
	}, nil
}

type addDelegatorRules struct {
	assetID                  ids.ID
	minDelegatorStake        uint64
	maxValidatorStake        uint64
	minStakeDuration         time.Duration
	maxStakeDuration         time.Duration
	maxValidatorWeightFactor byte
}

func getDelegatorRules(
	backend *Backend,
	chainState state.Chain,
	supernetID ids.ID,
) (*addDelegatorRules, error) {
	if supernetID == constants.PrimaryNetworkID {
		return &addDelegatorRules{
			assetID:                  backend.Ctx.JUNEAssetID,
			minDelegatorStake:        backend.Config.MinDelegatorStake,
			maxValidatorStake:        backend.Config.MaxValidatorStake,
			minStakeDuration:         backend.Config.MinStakeDuration,
			maxStakeDuration:         backend.Config.MaxStakeDuration,
			maxValidatorWeightFactor: MaxValidatorWeightFactor,
		}, nil
	}

	transformSupernet, err := GetTransformSupernetTx(chainState, supernetID)
	if err != nil {
		return nil, err
	}

	return &addDelegatorRules{
		assetID:                  transformSupernet.AssetID,
		minDelegatorStake:        transformSupernet.MinDelegatorStake,
		maxValidatorStake:        transformSupernet.MaxValidatorStake,
		minStakeDuration:         time.Duration(transformSupernet.MinStakeDuration) * time.Second,
		maxStakeDuration:         time.Duration(transformSupernet.MaxStakeDuration) * time.Second,
		maxValidatorWeightFactor: transformSupernet.MaxValidatorWeightFactor,
	}, nil
}

// GetNextStakerChangeTime returns the next time a staker will be either added
// or removed to/from the current validator set.
func GetNextStakerChangeTime(state state.Chain) (time.Time, error) {
	currentStakerIterator, err := state.GetCurrentStakerIterator()
	if err != nil {
		return time.Time{}, err
	}
	defer currentStakerIterator.Release()

	pendingStakerIterator, err := state.GetPendingStakerIterator()
	if err != nil {
		return time.Time{}, err
	}
	defer pendingStakerIterator.Release()

	hasCurrentStaker := currentStakerIterator.Next()
	hasPendingStaker := pendingStakerIterator.Next()
	switch {
	case hasCurrentStaker && hasPendingStaker:
		nextCurrentTime := currentStakerIterator.Value().NextTime
		nextPendingTime := pendingStakerIterator.Value().NextTime
		if nextCurrentTime.Before(nextPendingTime) {
			return nextCurrentTime, nil
		}
		return nextPendingTime, nil
	case hasCurrentStaker:
		return currentStakerIterator.Value().NextTime, nil
	case hasPendingStaker:
		return pendingStakerIterator.Value().NextTime, nil
	default:
		return time.Time{}, database.ErrNotFound
	}
}

// GetValidator returns information about the given validator, which may be a
// current validator or pending validator.
func GetValidator(state state.Chain, supernetID ids.ID, nodeID ids.NodeID) (*state.Staker, error) {
	validator, err := state.GetCurrentValidator(supernetID, nodeID)
	if err == nil {
		// This node is currently validating the supernet.
		return validator, nil
	}
	if err != database.ErrNotFound {
		// Unexpected error occurred.
		return nil, err
	}
	return state.GetPendingValidator(supernetID, nodeID)
}

// overDelegated returns true if [validator] will be overdelegated when adding [delegator].
//
// A [validator] would become overdelegated if:
// - the maximum total weight on [validator] exceeds [weightLimit]
func overDelegated(
	state state.Chain,
	validator *state.Staker,
	weightLimit uint64,
	delegatorWeight uint64,
	delegatorStartTime time.Time,
	delegatorEndTime time.Time,
) (bool, error) {
	maxWeight, err := GetMaxWeight(state, validator, delegatorStartTime, delegatorEndTime)
	if err != nil {
		return true, err
	}
	newMaxWeight, err := math.Add64(maxWeight, delegatorWeight)
	if err != nil {
		return true, err
	}
	return newMaxWeight > weightLimit, nil
}

// GetMaxWeight returns the maximum total weight of the [validator], including
// its own weight, between [startTime] and [endTime].
// The weight changes are applied in the order they will be applied as chain
// time advances.
// Invariant:
// - [validator.StartTime] <= [startTime] < [endTime] <= [validator.EndTime]
func GetMaxWeight(
	chainState state.Chain,
	validator *state.Staker,
	startTime time.Time,
	endTime time.Time,
) (uint64, error) {
	currentDelegatorIterator, err := chainState.GetCurrentDelegatorIterator(validator.SupernetID, validator.NodeID)
	if err != nil {
		return 0, err
	}

	// TODO: We can optimize this by moving the current total weight to be
	//       stored in the validator state.
	//
	// Calculate the current total weight on this validator, including the
	// weight of the actual validator and the sum of the weights of all of the
	// currently active delegators.
	currentWeight := validator.Weight
	for currentDelegatorIterator.Next() {
		currentDelegator := currentDelegatorIterator.Value()

		currentWeight, err = math.Add64(currentWeight, currentDelegator.Weight)
		if err != nil {
			currentDelegatorIterator.Release()
			return 0, err
		}
	}
	currentDelegatorIterator.Release()

	currentDelegatorIterator, err = chainState.GetCurrentDelegatorIterator(validator.SupernetID, validator.NodeID)
	if err != nil {
		return 0, err
	}
	pendingDelegatorIterator, err := chainState.GetPendingDelegatorIterator(validator.SupernetID, validator.NodeID)
	if err != nil {
		currentDelegatorIterator.Release()
		return 0, err
	}
	delegatorChangesIterator := state.NewStakerDiffIterator(currentDelegatorIterator, pendingDelegatorIterator)
	defer delegatorChangesIterator.Release()

	// Iterate over the future stake weight changes and calculate the maximum
	// total weight on the validator, only including the points in the time
	// range [startTime, endTime].
	var currentMax uint64
	for delegatorChangesIterator.Next() {
		delegator, isAdded := delegatorChangesIterator.Value()
		// [delegator.NextTime] > [endTime]
		if delegator.NextTime.After(endTime) {
			// This delegation change (and all following changes) occurs after
			// [endTime]. Since we're calculating the max amount staked in
			// [startTime, endTime], we can stop.
			break
		}

		// [delegator.NextTime] >= [startTime]
		if !delegator.NextTime.Before(startTime) {
			// We have advanced time to be at the inside of the delegation
			// window. Make sure that the max weight is updated accordingly.
			currentMax = max(currentMax, currentWeight)
		}

		var op func(uint64, uint64) (uint64, error)
		if isAdded {
			op = math.Add64
		} else {
			op = math.Sub[uint64]
		}
		currentWeight, err = op(currentWeight, delegator.Weight)
		if err != nil {
			return 0, err
		}
	}
	// Because we assume [startTime] < [endTime], we have advanced time to
	// be at the end of the delegation window. Make sure that the max weight is
	// updated accordingly.
	return max(currentMax, currentWeight), nil
}

func GetTransformSupernetTx(chain state.Chain, supernetID ids.ID) (*txs.TransformSupernetTx, error) {
	transformSupernetIntf, err := chain.GetSupernetTransformation(supernetID)
	if err != nil {
		return nil, err
	}

	transformSupernet, ok := transformSupernetIntf.Unsigned.(*txs.TransformSupernetTx)
	if !ok {
		return nil, ErrIsNotTransformSupernetTx
	}

	return transformSupernet, nil
}
