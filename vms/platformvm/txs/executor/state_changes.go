// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var (
	ErrChildBlockAfterStakerChangeTime = errors.New("proposed timestamp later than next staker change time")
	ErrChildBlockBeyondSyncBound       = errors.New("proposed timestamp is too far in the future relative to local time")
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
			ErrChildBlockAfterStakerChangeTime,
			newChainTime,
			nextStakerChangeTime,
		)
	}

	// Only allow timestamp to reasonably far forward
	maxNewChainTime := now.Add(SyncBound)
	if newChainTime.After(maxNewChainTime) {
		return fmt.Errorf(
			"%w, proposed time (%s), local time (%s)",
			ErrChildBlockBeyondSyncBound,
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
	updatedRewardPoolSupplies map[ids.ID]uint64
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
	for supernetID, rewardPoolSupply := range s.updatedRewardPoolSupplies {
		stateDiff.SetRewardPoolSupply(supernetID, rewardPoolSupply)
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
		updatedSupplies:           make(map[ids.ID]uint64),
		updatedRewardPoolSupplies: make(map[ids.ID]uint64),
	}

	// Add to the staker set any pending stakers whose start time is at or
	// before the new timestamp

	// Note: we process pending stakers ready to be promoted to current ones and
	// then we process current stakers to be demoted out of stakers set. It is
	// guaranteed that no promoted stakers would be demoted immediately. A
	// failure of this invariant would cause a staker to be added to
	// StateChanges and be persisted among current stakers even if it already
	// expired. The following invariants ensure this does not happens:
	// Invariant: minimum stake duration is > 0, so staker.StartTime != staker.EndTime.
	// Invariant: [newChainTime] does not skip stakers set change times.

	for pendingStakerIterator.Next() {
		stakerToRemove := pendingStakerIterator.Value()
		if stakerToRemove.StartTime.After(newChainTime) {
			break
		}

		stakerToAdd := *stakerToRemove
		stakerToAdd.NextTime = stakerToRemove.EndTime
		stakerToAdd.Priority = txs.PendingToCurrentPriorities[stakerToRemove.Priority]

		if stakerToRemove.Priority == txs.SupernetPermissionedValidatorPendingPriority {
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

		rewardPoolSupply := changes.updatedRewardPoolSupplies[stakerToRemove.SupernetID]
		if !ok {
			rewardPoolSupply, err = parentState.GetRewardPoolSupply(stakerToRemove.SupernetID)
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
				stakerToRemove.StartTime,
				stakerToRemove.Weight,
			)
		} else {
			potentialReward = rewards.Calculate(
				stakerToRemove.EndTime.Sub(stakerToRemove.StartTime),
				stakerToRemove.StartTime,
				stakerToRemove.Weight,
				rewardPoolSupply,
			)
		}
		stakerToAdd.PotentialReward = potentialReward

		// Reward value above reward pool supply.
		extraValue := uint64(0)

		if stakerToRemove.SupernetID == constants.PrimaryNetworkID {
			if potentialReward > rewardPoolSupply {
				extraValue = potentialReward - rewardPoolSupply
			}
			if extraValue > 0 {
				// Extra value will be minted update supply accordingly.
				supply, err = math.Add64(supply, extraValue)
				if err != nil {
					return nil, err
				}
			}
		}

		rewardPoolSupply, err = math.Sub(rewardPoolSupply, potentialReward-extraValue)
		if err != nil {
			return nil, err
		}
		changes.updatedRewardPoolSupplies[stakerToRemove.SupernetID] = rewardPoolSupply

		changes.updatedSupplies[stakerToRemove.SupernetID] = supply

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

	transformSupernet, err := GetTransformSupernetTx(parentState, supernetID)
	if err != nil {
		return nil, err
	}

	return reward.NewCalculator(reward.Config{
		MinStakePeriod:         time.Duration(transformSupernet.MinStakeDuration),
		MaxStakePeriod:         time.Duration(transformSupernet.MaxStakeDuration),
		StakePeriodRewardShare: transformSupernet.StakePeriodRewardShare,
		StartRewardShare:       transformSupernet.StartRewardShare,
		StartRewardTime:        transformSupernet.StartRewardTime,
		TargetRewardShare:      transformSupernet.TargetRewardShare,
		TargetRewardTime:       transformSupernet.TargetRewardTime,
	}), nil
}
