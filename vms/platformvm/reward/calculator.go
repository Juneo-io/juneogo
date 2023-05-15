// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"
)

var _ Calculator = (*calculator)(nil)

type Calculator interface {
	Calculate(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
	CalculatePrimary(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64
}

type calculator struct {
	maxSubMinConsumptionRate *big.Int
	minConsumptionRate       *big.Int
	mintingPeriod            *big.Int
	supplyCap                uint64
}

func NewCalculator(c Config) Calculator {
	return &calculator{
		maxSubMinConsumptionRate: new(big.Int).SetUint64(c.MaxConsumptionRate - c.MinConsumptionRate),
		minConsumptionRate:       new(big.Int).SetUint64(c.MinConsumptionRate),
		mintingPeriod:            new(big.Int).SetUint64(uint64(c.MintingPeriod)),
		supplyCap:                c.SupplyCap,
	}
}

var (
	y1      = uint64(time.Date(2023, time.June, 21, 0, 0, 0, 0, time.UTC).Unix())
	y5      = uint64(time.Date(2027, time.June, 21, 0, 0, 0, 0, time.UTC).Unix())
	y6      = uint64(time.Date(2028, time.June, 21, 0, 0, 0, 0, time.UTC).Unix())
	y1Value = uint64(215000) // 23.5% - 2% = 21.5%
	y5Value = uint64(195000) // 21.5% - 2% = 19.5%
	y6Value = uint64(65000)  // 8.5% - 2% = 6.5%
)

// Reward returns the amount of tokens to reward the staker with in a permissionless supernet.
func (c *calculator) Calculate(stakedDuration time.Duration, currentTime time.Time, stakedAmount uint64) uint64 {
	timePercentage := new(big.Int).SetUint64(uint64(stakedDuration))
	timePercentage.Mul(timePercentage, rewardShareDenominator)
	timePercentage.Div(timePercentage, c.mintingPeriod)
	bonusRewards := new(big.Int).SetUint64(uint64(stakedDuration))
	bonusRewards.Mul(bonusRewards, rewardShareDenominator)
	bonusRewards.Div(bonusRewards, c.mintingPeriod)
	bonusRewards.Mul(bonusRewards, maxBonusRewardShare)
	bonusRewards.Div(bonusRewards, rewardShareDenominator)
	return GetTimeRewardsValue(rewardShare, rewardShare, bonusRewards, timePercentage, rewardShareDenominator, stakedAmount).Uint64()
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
	if currentTimeValue >= y6 {
		return GetTimeRewardsValue(y6Value, y6Value, bonusRewards, timePercentage, rewardShareDenominator, stakedAmount)
	}
	if currentTimeValue >= y5 {
		return GetTimeRewardsValue(y6Value, y5Value, bonusRewards, timePercentage, GetTimeBoundsPercentage(y6, y5, currentTimeValue), stakedAmount)
	}
	if currentTimeValue >= y1 {
		return GetTimeRewardsValue(y5Value, y1Value, bonusRewards, timePercentage, GetTimeBoundsPercentage(y5, y1, currentTimeValue), stakedAmount)
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
