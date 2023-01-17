// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
	"github.com/Juneo-io/juneogo/vms/relayvm/state"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

var (
	errChildBlockAfterStakerChangeTime = errors.New("proposed timestamp later than next staker change time")
	errChildBlockBeyondSyncBound       = errors.New("proposed timestamp is too far in the future relative to local time")
)

// VerifyNewChainTime returns nil if the [newChainTime] is a valid chain time
// given the wall clock time ([now]) and when the next staking set change occurs
// ([nextStakerChangeTime]).
// Requires:
//   - [newChainTime] <= [nextStakerChangeTime]: so that no staking set changes
//     are skipped.
//   - [newChainTime] <= [now] + [SyncBound]: to ensure chain time approximates
//     "real" time.
func VerifyNewChainTime(
	newChainTime,
	nextStakerChangeTime,
	now time.Time,
) error {
	// Only allow timestamp to move as far forward as the time of the next
	// staker set change
	if newChainTime.After(nextStakerChangeTime) {
		return fmt.Errorf(
			"%w, proposed timestamp (%s), next staker change time (%s)",
			errChildBlockAfterStakerChangeTime,
			newChainTime,
			nextStakerChangeTime,
		)
	}

	// Only allow timestamp to reasonably far forward
	maxNewChainTime := now.Add(SyncBound)
	if newChainTime.After(maxNewChainTime) {
		return fmt.Errorf(
			"%w, proposed time (%s), local time (%s)",
			errChildBlockBeyondSyncBound,
			newChainTime,
			now,
		)
	}
	return nil
}

type StateChanges interface {
	Apply(onAccept state.Diff)
	Len() int
}

type stateChanges struct {
	updatedSupplies           map[ids.ID]uint64
	updatedRewardsSupplies    map[ids.ID]uint64
	currentValidatorsToAdd    []*state.Staker
	currentDelegatorsToAdd    []*state.Staker
	pendingValidatorsToRemove []*state.Staker
	pendingDelegatorsToRemove []*state.Staker
	currentValidatorsToRemove []*state.Staker
}

func (s *stateChanges) Apply(stateDiff state.Diff) {
	for supernetID, supply := range s.updatedSupplies {
		stateDiff.SetCurrentSupply(supernetID, supply)
	}
	for supernetID, rewardsSupply := range s.updatedRewardsSupplies {
		stateDiff.SetRewardsPoolSupply(supernetID, rewardsSupply)
	}

	for _, currentValidatorToAdd := range s.currentValidatorsToAdd {
		stateDiff.PutCurrentValidator(currentValidatorToAdd)
	}
	for _, pendingValidatorToRemove := range s.pendingValidatorsToRemove {
		stateDiff.DeletePendingValidator(pendingValidatorToRemove)
	}
	for _, currentDelegatorToAdd := range s.currentDelegatorsToAdd {
		stateDiff.PutCurrentDelegator(currentDelegatorToAdd)
	}
	for _, pendingDelegatorToRemove := range s.pendingDelegatorsToRemove {
		stateDiff.DeletePendingDelegator(pendingDelegatorToRemove)
	}
	for _, currentValidatorToRemove := range s.currentValidatorsToRemove {
		stateDiff.DeleteCurrentValidator(currentValidatorToRemove)
	}
}

func (s *stateChanges) Len() int {
	return len(s.currentValidatorsToAdd) + len(s.currentDelegatorsToAdd) +
		len(s.pendingValidatorsToRemove) + len(s.pendingDelegatorsToRemove) +
		len(s.currentValidatorsToRemove)
}

// AdvanceTimeTo does not modify [parentState].
// Instead it returns all the StateChanges caused by advancing the chain time to
// the [newChainTime].
func AdvanceTimeTo(
	backend *Backend,
	parentState state.Chain,
	newChainTime time.Time,
) (StateChanges, error) {
	pendingStakerIterator, err := parentState.GetPendingStakerIterator()
	if err != nil {
		return nil, err
	}
	defer pendingStakerIterator.Release()

	changes := &stateChanges{
		updatedRewardsSupplies: make(map[ids.ID]uint64),
	}

	totalExtraValue := uint64(0)
	// Add to the staker set any pending stakers whose start time is at or
	// before the new timestamp
	for pendingStakerIterator.Next() {
		stakerToRemove := pendingStakerIterator.Value()
		if stakerToRemove.StartTime.After(newChainTime) {
			break
		}

		stakerToAdd := *stakerToRemove
		stakerToAdd.NextTime = stakerToRemove.EndTime
		stakerToAdd.Priority = txs.PendingToCurrentPriorities[stakerToRemove.Priority]

		if stakerToRemove.Priority == txs.SupernetPermissionedValidatorPendingPriority {
			// Invariant: [txTimestamp] <= [nextStakerChangeTime].
			// Invariant: minimum stake duration is > 0.
			//
			// Both of the above invariants ensure the staker we are adding here
			// should never be attempted to be removed in the following loop.

			changes.currentValidatorsToAdd = append(changes.currentValidatorsToAdd, &stakerToAdd)
			changes.pendingValidatorsToRemove = append(changes.pendingValidatorsToRemove, stakerToRemove)
			continue
		}

		supply, ok := changes.updatedSupplies[stakerToRemove.SupernetID]
		if !ok {
			supply, err = parentState.GetCurrentSupply(stakerToRemove.SupernetID)
			if err != nil {
				return nil, err
			}
		}

		rewardsSupply, ok := changes.updatedRewardsSupplies[stakerToRemove.SupernetID]
		if !ok {
			rewardsSupply, err = parentState.GetRewardsPoolSupply(stakerToRemove.SupernetID)
			if err != nil {
				return nil, err
			}
		}

		rewards, err := GetRewardsCalculator(backend, parentState, stakerToRemove.SupernetID)
		if err != nil {
			return nil, err
		}

		potentialReward := uint64(0)
		if stakerToRemove.SupernetID == constants.PrimaryNetworkID {
			potentialReward = rewards.CalculatePrimary(
				stakerToRemove.EndTime.Sub(stakerToRemove.StartTime),
				newChainTime,
				stakerToRemove.Weight,
			)
		} else {
			potentialReward = rewards.Calculate_(
				stakerToRemove.EndTime.Sub(stakerToRemove.StartTime),
				newChainTime,
				stakerToRemove.Weight,
			)
		}
		extraValue := uint64(0)
		if potentialReward > rewardsSupply {
			extraValue = potentialReward - rewardsSupply
		}
		rewardsSupply, err = math.Sub(rewardsSupply, potentialReward-extraValue)
		if extraValue > 0 {
			supply, err = math.Add64(supply, extraValue)
			if err != nil {
				pendingStakerIterator.Release()
				return nil, err
			}
			totalExtraValue, err = math.Add64(totalExtraValue, extraValue)
			if err != nil {
				pendingStakerIterator.Release()
				return nil, err
			}
		}
		stakerToAdd.PotentialReward = potentialReward

		changes.updatedRewardsSupplies[stakerToRemove.SupernetID] = rewardsSupply
		if totalExtraValue > 0 {
			changes.updatedSupplies = make(map[ids.ID]uint64)
			changes.updatedSupplies[stakerToRemove.SupernetID] = supply
		}

		switch stakerToRemove.Priority {
		case txs.PrimaryNetworkValidatorPendingPriority, txs.SupernetPermissionlessValidatorPendingPriority:
			changes.currentValidatorsToAdd = append(changes.currentValidatorsToAdd, &stakerToAdd)
			changes.pendingValidatorsToRemove = append(changes.pendingValidatorsToRemove, stakerToRemove)

		case txs.PrimaryNetworkDelegatorApricotPendingPriority, txs.PrimaryNetworkDelegatorBanffPendingPriority, txs.SupernetPermissionlessDelegatorPendingPriority:
			changes.currentDelegatorsToAdd = append(changes.currentDelegatorsToAdd, &stakerToAdd)
			changes.pendingDelegatorsToRemove = append(changes.pendingDelegatorsToRemove, stakerToRemove)

		default:
			return nil, fmt.Errorf("expected staker priority got %d", stakerToRemove.Priority)
		}
	}

	currentStakerIterator, err := parentState.GetCurrentStakerIterator()
	if err != nil {
		return nil, err
	}
	defer currentStakerIterator.Release()

	for currentStakerIterator.Next() {
		stakerToRemove := currentStakerIterator.Value()
		if stakerToRemove.EndTime.After(newChainTime) {
			break
		}

		// Invariant: Permissioned stakers are encountered first for a given
		//            timestamp because their priority is the smallest.
		if stakerToRemove.Priority != txs.SupernetPermissionedValidatorCurrentPriority {
			// Permissionless stakers are removed by the RewardValidatorTx, not
			// an AdvanceTimeTx.
			break
		}

		changes.currentValidatorsToRemove = append(changes.currentValidatorsToRemove, stakerToRemove)
	}
	return changes, nil
}

func GetRewardsCalculator(
	backend *Backend,
	parentState state.Chain,
	supernetID ids.ID,
) (reward.Calculator, error) {
	if supernetID == constants.PrimaryNetworkID {
		return backend.Rewards, nil
	}

	transformSupernetIntf, err := parentState.GetSupernetTransformation(supernetID)
	if err != nil {
		return nil, err
	}
	transformSupernet, ok := transformSupernetIntf.Unsigned.(*txs.TransformSupernetTx)
	if !ok {
		return nil, errIsNotTransformSupernetTx
	}

	return reward.NewCalculator(reward.Config{
		MintingPeriod: backend.Config.RewardConfig.MintingPeriod,
		RewardShare:   transformSupernet.RewardShare,
	}), nil
}
