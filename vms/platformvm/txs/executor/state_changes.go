// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
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

func NextBlockTime(state state.Chain, clk *mockable.Clock) (time.Time, bool, error) {
	var (
		timestamp  = clk.Time()
		parentTime = state.GetTimestamp()
	)
	if parentTime.After(timestamp) {
		timestamp = parentTime
	}
	// [timestamp] = max(now, parentTime)

	nextStakerChangeTime, err := GetNextStakerChangeTime(state)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("failed getting next staker change time: %w", err)
	}

	// timeWasCapped means that [timestamp] was reduced to [nextStakerChangeTime]
	timeWasCapped := !timestamp.Before(nextStakerChangeTime)
	if timeWasCapped {
		timestamp = nextStakerChangeTime
	}
	// [timestamp] = min(max(now, parentTime), nextStakerChangeTime)
	return timestamp, timeWasCapped, nil
}

// AdvanceTimeTo applies all state changes to [parentState] resulting from
// advancing the chain time to [newChainTime].
// Returns true iff the validator set changed.
func AdvanceTimeTo(
	backend *Backend,
	parentState state.Chain,
	newChainTime time.Time,
) (bool, error) {
	// We promote pending stakers to current stakers first and remove
	// completed stakers from the current staker set. We assume that any
	// promoted staker will not immediately be removed from the current staker
	// set. This is guaranteed by the following invariants.
	//
	// Invariant: MinStakeDuration > 0 => guarantees [StartTime] != [EndTime]
	// Invariant: [newChainTime] <= nextStakerChangeTime.

	changes, err := state.NewDiffOn(parentState)
	if err != nil {
		return false, err
	}

	pendingStakerIterator, err := parentState.GetPendingStakerIterator()
	if err != nil {
		return false, err
	}
	defer pendingStakerIterator.Release()

	var changed bool
	// Promote any pending stakers to current if [StartTime] <= [newChainTime].
	for pendingStakerIterator.Next() {
		stakerToRemove := pendingStakerIterator.Value()
		if stakerToRemove.StartTime.After(newChainTime) {
			break
		}

		stakerToAdd := *stakerToRemove
		stakerToAdd.NextTime = stakerToRemove.EndTime
		stakerToAdd.Priority = txs.PendingToCurrentPriorities[stakerToRemove.Priority]

		if stakerToRemove.Priority == txs.SupernetPermissionedValidatorPendingPriority {
			changes.PutCurrentValidator(&stakerToAdd)
			changes.DeletePendingValidator(stakerToRemove)
			changed = true
			continue
		}

		supply, err := changes.GetCurrentSupply(stakerToRemove.SupernetID)
		if err != nil {
			return false, err
		}

		rewardPoolSupply, err := changes.GetRewardPoolSupply(stakerToRemove.SupernetID)
		if err != nil {
			return false, err
		}

		rewards, err := GetRewardsCalculator(backend, parentState, stakerToRemove.SupernetID)
		if err != nil {
			return false, err
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
					return false, err
				}
			}
		}

		rewardPoolSupply, err = math.Sub(rewardPoolSupply, potentialReward-extraValue)
		if err != nil {
			return false, err
		}
		changes.SetRewardPoolSupply(stakerToRemove.SupernetID, rewardPoolSupply)

		changes.SetCurrentSupply(stakerToRemove.SupernetID, supply)

		switch stakerToRemove.Priority {
		case txs.PrimaryNetworkValidatorPendingPriority, txs.SupernetPermissionlessValidatorPendingPriority:
			changes.PutCurrentValidator(&stakerToAdd)
			changes.DeletePendingValidator(stakerToRemove)

		case txs.PrimaryNetworkDelegatorApricotPendingPriority, txs.PrimaryNetworkDelegatorBanffPendingPriority, txs.SupernetPermissionlessDelegatorPendingPriority:
			changes.PutCurrentDelegator(&stakerToAdd)
			changes.DeletePendingDelegator(stakerToRemove)

		default:
			return false, fmt.Errorf("expected staker priority got %d", stakerToRemove.Priority)
		}

		changed = true
	}

	// Remove any current stakers whose [EndTime] <= [newChainTime].
	currentStakerIterator, err := parentState.GetCurrentStakerIterator()
	if err != nil {
		return false, err
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

		changes.DeleteCurrentValidator(stakerToRemove)
		changed = true
	}

	if err := changes.Apply(parentState); err != nil {
		return false, err
	}

	parentState.SetTimestamp(newChainTime)
	return changed, nil
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
		DiminishingRewardShare: transformSupernet.DiminishingRewardShare,
		DiminishingRewardTime:  transformSupernet.DiminishingRewardTime,
		TargetRewardShare:      transformSupernet.TargetRewardShare,
		TargetRewardTime:       transformSupernet.TargetRewardTime,
	}), nil
}
