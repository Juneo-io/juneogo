// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"

	"github.com/ava-labs/avalanchego/utils/math"
)

var _ Calculator = (*calculator)(nil)

type Calculator interface {
	Calculate(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64, rewardPoolValue uint64) uint64
	CalculatePrimary(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
}

type calculator struct {
	minStakePeriod         uint64
	maxStakePeriod         uint64
	stakePeriodRewardShare uint64
	startRewardShare       uint64
	startRewardTime        uint64
	targetRewardShare      uint64
	targetRewardTime       uint64
}

func NewCalculator(c Config) Calculator {
	return &calculator{
		minStakePeriod:         uint64(c.MinStakePeriod),
		maxStakePeriod:         uint64(c.MaxStakePeriod),
		stakePeriodRewardShare: c.StakePeriodRewardShare,
		startRewardShare:       c.StartRewardShare,
		startRewardTime:        uint64(c.StartRewardTime.Unix()),
		targetRewardShare:      c.TargetRewardShare,
		targetRewardTime:       uint64(c.TargetRewardTime.Unix()),
	}
}

var (
	DiminishingRewardTime  = uint64(time.Date(2027, time.June, 21, 0, 0, 0, 0, time.UTC).Unix())
	DiminishingRewardShare = uint64(19_5000)
)

func (c *calculator) Calculate(stakedDuration time.Duration, currentTime time.Time, stakeAmount uint64, rewardPoolValue uint64) uint64 {
	boundsPercentage := getRemainingTimeBoundsPercentage(c.startRewardTime, c.targetRewardTime, uint64(currentTime.Unix()))
	reward := getReward(c.targetRewardShare, c.startRewardShare, boundsPercentage)
	effectiveReward := c.getEffectiveReward(uint64(stakedDuration), stakeAmount, reward)
	if effectiveReward > rewardPoolValue {
		return rewardPoolValue
	}
	return effectiveReward
}

func (c *calculator) CalculatePrimary(stakedDuration time.Duration, currentTime time.Time, stakeAmount uint64) uint64 {
	reward := c.getCurrentPrimaryReward(uint64(currentTime.Unix()))
	return c.getEffectiveReward(uint64(stakedDuration), stakeAmount, reward)
}

func (c *calculator) getEffectiveReward(stakePeriod uint64, stakeAmount uint64, reward *big.Int) uint64 {
	stakePeriodBig := new(big.Int).SetUint64(stakePeriod)
	reward.Add(reward, c.getStakePeriodReward(stakePeriod))
	stakePeriodRatio := stakePeriodBig.Mul(stakePeriodBig, rewardShareDenominator)
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
	stakePeriodBig := new(big.Int).SetUint64(stakePeriod)
	minStakePeriodBig := new(big.Int).SetUint64(c.minStakePeriod)
	maxStakePeriodBig := new(big.Int).SetUint64(c.maxStakePeriod)
	adjustedStakePeriod := stakePeriodBig.Sub(stakePeriodBig, minStakePeriodBig)
	adjustedStakePeriod.Mul(adjustedStakePeriod, rewardShareDenominator)
	adjustedMaxStakePeriod := maxStakePeriodBig.Sub(maxStakePeriodBig, minStakePeriodBig)
	reward := adjustedStakePeriod.Div(adjustedStakePeriod, adjustedMaxStakePeriod)
	reward.Mul(reward, new(big.Int).SetUint64(c.stakePeriodRewardShare))
	reward.Div(reward, rewardShareDenominator)
	return reward
}

func (c *calculator) getCurrentPrimaryReward(currentTime uint64) *big.Int {
	if currentTime >= c.targetRewardTime {
		return new(big.Int).SetUint64(c.targetRewardShare)
	}
	if currentTime >= DiminishingRewardTime {
		reward := getReward(
			c.targetRewardShare,
			DiminishingRewardShare,
			getRemainingTimeBoundsPercentage(DiminishingRewardTime, c.targetRewardTime, currentTime),
		)
		if reward.IsUint64() {
			return reward
		} else {
			return new(big.Int).SetUint64(uint64(0))
		}
	}
	if currentTime >= c.startRewardTime {
		reward := getReward(
			DiminishingRewardShare,
			c.startRewardShare,
			getRemainingTimeBoundsPercentage(c.startRewardTime, DiminishingRewardTime, currentTime),
		)
		if reward.IsUint64() {
			return reward
		} else {
			return new(big.Int).SetUint64(uint64(0))
		}
	}
	// Start period or before
	return new(big.Int).SetUint64(c.startRewardShare)
}

func getReward(lowerReward uint64, upperReward uint64, remainingTimeBoundsPercentage *big.Int) *big.Int {
	diminishingReward, err := math.Sub(upperReward, lowerReward)
	if err != nil {
		diminishingReward = uint64(0)
	}
	diminishingRewardBig := new(big.Int).SetUint64(diminishingReward)
	remainingReward := diminishingRewardBig.Mul(diminishingRewardBig, remainingTimeBoundsPercentage)
	remainingReward.Div(remainingReward, rewardShareDenominator)
	lowerRewardBig := new(big.Int).SetUint64(lowerReward)
	remainingReward.Add(remainingReward, lowerRewardBig)
	return remainingReward
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
	elapsedTimeBig := new(big.Int).SetUint64(elapsedTime)
	maxElapsedTimeBig := new(big.Int).SetUint64(maxElapsedTime)
	elapsedRatio := elapsedTimeBig.Mul(elapsedTimeBig, rewardShareDenominator)
	elapsedRatio.Div(elapsedRatio, maxElapsedTimeBig)
	return rewardShareDenominator.Sub(rewardShareDenominator, elapsedRatio)
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
