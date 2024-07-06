// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"

	"github.com/Juneo-io/juneogo/utils/math"
)

var _ Calculator = (*calculator)(nil)

type Calculator interface {
	Calculate(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
}

type calculator struct {
	minStakePeriod         uint64
	maxStakePeriod         uint64
	stakePeriodRewardShare uint64
	startRewardShare       uint64
	startRewardTime        uint64
	diminishingRewardShare uint64
	diminishingRewardTime  uint64
	targetRewardShare      uint64
	targetRewardTime       uint64
}

func NewCalculator(c Config) Calculator {
	return &calculator{
		minStakePeriod:         uint64(c.MinStakePeriod),
		maxStakePeriod:         uint64(c.MaxStakePeriod),
		stakePeriodRewardShare: c.StakePeriodRewardShare,
		startRewardShare:       c.StartRewardShare,
		startRewardTime:        c.StartRewardTime,
		diminishingRewardShare: c.DiminishingRewardShare,
		diminishingRewardTime:  c.DiminishingRewardTime,
		targetRewardShare:      c.TargetRewardShare,
		targetRewardTime:       c.TargetRewardTime,
	}
}

func (c *calculator) Calculate(stakedDuration time.Duration, currentTime time.Time, stakeAmount uint64) uint64 {
	reward := c.getCurrentReward(uint64(currentTime.Unix()))
	stakePeriod := uint64(stakedDuration)
	reward.Add(reward, c.getStakePeriodReward(stakePeriod))
	stakePeriodRatio := new(big.Int).SetUint64(stakePeriod)
	stakePeriodRatio.Mul(stakePeriodRatio, rewardShareDenominator)
	stakePeriodRatio.Div(stakePeriodRatio, new(big.Int).SetUint64(c.maxStakePeriod))
	effectiveReward := reward.Mul(reward, stakePeriodRatio)
	effectiveReward.Div(effectiveReward, rewardShareDenominator)
	effectiveReward.Mul(effectiveReward, new(big.Int).SetUint64(stakeAmount))
	effectiveReward.Div(effectiveReward, rewardShareDenominator)
	if !effectiveReward.IsUint64() {
		return uint64(0)
	}
	return effectiveReward.Uint64()
}

func (c *calculator) getStakePeriodReward(stakePeriod uint64) *big.Int {
	minStakePeriodBig := new(big.Int).SetUint64(c.minStakePeriod)
	adjustedStakePeriod := new(big.Int).SetUint64(stakePeriod)
	adjustedStakePeriod.Sub(adjustedStakePeriod, minStakePeriodBig)
	adjustedStakePeriod.Mul(adjustedStakePeriod, rewardShareDenominator)
	adjustedMaxStakePeriod := new(big.Int).SetUint64(c.maxStakePeriod)
	adjustedMaxStakePeriod.Sub(adjustedMaxStakePeriod, minStakePeriodBig)
	reward := adjustedStakePeriod.Div(adjustedStakePeriod, adjustedMaxStakePeriod)
	reward.Mul(reward, new(big.Int).SetUint64(c.stakePeriodRewardShare))
	reward.Div(reward, rewardShareDenominator)
	return reward
}

func (c *calculator) getCurrentReward(currentTime uint64) *big.Int {
	if currentTime >= c.targetRewardTime {
		return new(big.Int).SetUint64(c.targetRewardShare)
	}
	if currentTime >= c.diminishingRewardTime {
		return getReward(
			c.targetRewardShare,
			c.diminishingRewardShare,
			getRemainingTimeBoundsPercentage(c.diminishingRewardTime, c.targetRewardTime, currentTime),
		)
	}
	if currentTime >= c.startRewardTime {
		return getReward(
			c.diminishingRewardShare,
			c.startRewardShare,
			getRemainingTimeBoundsPercentage(c.startRewardTime, c.diminishingRewardTime, currentTime),
		)
	}
	// Start period or before
	return new(big.Int).SetUint64(c.startRewardShare)
}

func getReward(lowerReward uint64, upperReward uint64, remainingTimeBoundsPercentage *big.Int) *big.Int {
	diminishingReward, err := math.Sub(upperReward, lowerReward)
	if err != nil {
		diminishingReward = uint64(0)
	}
	remainingReward := new(big.Int).SetUint64(diminishingReward)
	remainingReward.Mul(remainingReward, remainingTimeBoundsPercentage)
	remainingReward.Div(remainingReward, rewardShareDenominator)
	return remainingReward.Add(remainingReward, new(big.Int).SetUint64(lowerReward))
}

// The remaining percentage between lower and upper bounds calculated against current time.
// Returned value is [PercentDenominator, 0]. If currentTime is out of upper bound then
// 0 is returned. If currentTime is out of lower bound then PercentDenominator (100%) is returned.
func getRemainingTimeBoundsPercentage(lowerTimeBound uint64, upperTimeBound uint64, currentTime uint64) *big.Int {
	// Current time is before or at lower bound
	if currentTime <= lowerTimeBound {
		return rewardShareDenominator
	}
	maxElapsedTime, err := math.Sub(upperTimeBound, lowerTimeBound)
	if err != nil {
		return new(big.Int).SetUint64(uint64(0))
	}
	elapsedTime, err := math.Sub(currentTime, lowerTimeBound)
	if err != nil {
		return new(big.Int).SetUint64(uint64(0))
	}
	// Current time is after or at upper bound
	if elapsedTime >= maxElapsedTime {
		return new(big.Int).SetUint64(uint64(0))
	}
	maxElapsedTimeBig := new(big.Int).SetUint64(maxElapsedTime)
	elapsedRatio := new(big.Int).SetUint64(elapsedTime)
	elapsedRatio.Mul(elapsedRatio, rewardShareDenominator)
	elapsedRatio.Div(elapsedRatio, maxElapsedTimeBig)
	remaining := new(big.Int).SetUint64(PercentDenominator)
	return remaining.Sub(remaining, elapsedRatio)
}

// Split [totalAmount] into [totalAmount * shares percentage] and the remainder.
//
// Invariant: [shares] <= [PercentDenominator]
func Split(totalAmount uint64, shares uint32) (uint64, uint64) {
	remainderShares := PercentDenominator - uint64(shares)
	remainderAmount := remainderShares * (totalAmount / PercentDenominator)

	// Delay rounding as long as possible for small numbers
	if optimisticReward, err := math.Mul64(remainderShares, totalAmount); err == nil {
		remainderAmount = optimisticReward / PercentDenominator
	}

	amountFromShares := totalAmount - remainderAmount
	return amountFromShares, remainderAmount
}
