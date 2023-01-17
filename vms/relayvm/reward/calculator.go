// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"
)

var _ Calculator = (*calculator)(nil)

type Calculator interface {
	Calculate_(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
	CalculatePrimary(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
}

type calculator struct {
	mintingPeriod *big.Int
	rewardShare   uint64
}

func NewCalculator(c Config) Calculator {
	return &calculator{
		mintingPeriod: new(big.Int).SetUint64(uint64(c.MintingPeriod)),
		rewardShare:   uint64(c.RewardShare),
	}
}

var (
	y7      = uint64(time.Date(2028, time.September, 1, 0, 0, 0, 0, time.UTC).Unix())
	y6      = uint64(time.Date(2027, time.September, 1, 0, 0, 0, 0, time.UTC).Unix())
	y4      = uint64(time.Date(2025, time.September, 1, 0, 0, 0, 0, time.UTC).Unix())
	y1      = uint64(time.Date(2022, time.September, 1, 0, 0, 0, 0, time.UTC).Unix())
	y7Value = uint64(50000)  // 5%
	y6Value = uint64(87500)  // 8.75%
	y4Value = uint64(167500) // 16.75%
	y1Value = uint64(227500) // 22.75%
)

// Reward returns the amount of tokens to reward the staker with in a permissionless supernet.
func (c *calculator) Calculate_(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64 {
	timePercentage := new(big.Int).SetUint64(uint64(stakedDuration))
	timePercentage.Mul(timePercentage, rewardShareDenominator)
	timePercentage.Div(timePercentage, c.mintingPeriod)
	bonusRewards := new(big.Int).SetUint64(uint64(stakedDuration))
	bonusRewards.Mul(bonusRewards, rewardShareDenominator)
	bonusRewards.Div(bonusRewards, c.mintingPeriod)
	bonusRewards.Mul(bonusRewards, maxBonusRewardShare)
	bonusRewards.Div(bonusRewards, rewardShareDenominator)
	return GetTimeRewardsValue(c.rewardShare, c.rewardShare, bonusRewards, timePercentage, rewardShareDenominator, stakedAmount).Uint64()
}

// Reward returns the amount of tokens to reward the staker with in the primary supernet.
func (c *calculator) CalculatePrimary(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64 {
	timePercentage := new(big.Int).SetUint64(uint64(stakedDuration))
	timePercentage.Mul(timePercentage, rewardShareDenominator)
	timePercentage.Div(timePercentage, c.mintingPeriod)
	bonusRewards := new(big.Int).SetUint64(uint64(stakedDuration))
	bonusRewards.Mul(bonusRewards, rewardShareDenominator)
	bonusRewards.Div(bonusRewards, c.mintingPeriod)
	bonusRewards.Mul(bonusRewards, maxBonusRewardShare)
	bonusRewards.Div(bonusRewards, rewardShareDenominator)
	return GetTimeRewards(currentTime, stakedAmount, bonusRewards, timePercentage).Uint64()
}

func GetTimeRewards(currentTime time.Time, stakedAmount uint64, bonusRewards *big.Int, timePercentage *big.Int) *big.Int {
	currentTimeValue := uint64(currentTime.Unix())
	if currentTimeValue >= y7 {
		return GetTimeRewardsValue(y7Value, y7Value, bonusRewards, timePercentage, rewardShareDenominator, stakedAmount)
	}
	if currentTimeValue >= y6 {
		return GetTimeRewardsValue(y7Value, y6Value, bonusRewards, timePercentage, GetTimeBoundsPercentage(y7, y6, currentTimeValue), stakedAmount)
	}
	if currentTimeValue >= y4 {
		return GetTimeRewardsValue(y6Value, y4Value, bonusRewards, timePercentage, GetTimeBoundsPercentage(y6, y4, currentTimeValue), stakedAmount)
	}
	if currentTimeValue >= y1 {
		return GetTimeRewardsValue(y4Value, y1Value, bonusRewards, timePercentage, GetTimeBoundsPercentage(y4, y1, currentTimeValue), stakedAmount)
	}
	return GetTimeRewardsValue(y1Value, y1Value, bonusRewards, timePercentage, rewardShareDenominator, stakedAmount)
}

func GetTimeRewardsValue(lowerValue uint64, upperValue uint64, bonusRewards *big.Int, timePercentage *big.Int, timeBoundsPercentage *big.Int, stakedAmount uint64) *big.Int {
	bigLowerValue := new(big.Int).SetUint64(lowerValue)
	bigLowerValue.Add(bigLowerValue, rewardShareDenominator)
	bigRewardsValue := new(big.Int).SetUint64(upperValue)
	bigRewardsValue.Add(bigRewardsValue, rewardShareDenominator)
	bigStakedAmount := new(big.Int).SetUint64(stakedAmount)
	bigRewardsValue.Sub(bigRewardsValue, bigLowerValue)
	bigRewardsValue.Mul(bigRewardsValue, timeBoundsPercentage)
	bigRewardsValue.Div(bigRewardsValue, rewardShareDenominator)
	bigRewardsValue.Add(bigRewardsValue, bigLowerValue)
	bigRewardsValue.Add(bigRewardsValue, bonusRewards)
	bigRewardsValue.Mul(bigRewardsValue, bigStakedAmount)
	bigRewardsValue.Div(bigRewardsValue, rewardShareDenominator)
	bigRewardsValue.Sub(bigRewardsValue, bigStakedAmount)
	bigRewardsValue.Mul(bigRewardsValue, timePercentage)
	bigRewardsValue.Div(bigRewardsValue, rewardShareDenominator)
	return bigRewardsValue
}

func GetTimeBoundsPercentage(lowerTimeBound uint64, upperTimeBound uint64, currentTimeValue uint64) *big.Int {
	bigLowerBound := new(big.Int).SetUint64(lowerTimeBound)
	bigPeriodValue := new(big.Int).SetUint64(currentTimeValue)
	bigPeriodValueDenominator := new(big.Int).SetUint64(upperTimeBound)
	bigPeriodValue.Sub(bigPeriodValue, bigLowerBound)
	bigPeriodValueDenominator.Sub(bigPeriodValueDenominator, bigLowerBound)
	bigPeriodValue.Mul(bigPeriodValue, rewardShareDenominator)
	bigPeriodValue.Div(bigPeriodValue, bigPeriodValueDenominator)
	return bigPeriodValue
}
