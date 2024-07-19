// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/utils/units"
)

const (
	defaultMinStakingDuration = 24 * time.Hour
	defaultMaxStakingDuration = 365 * 24 * time.Hour

	defaultMinValidatorStake = 5 * units.MilliAvax
)

var defaultConfig = Config{
	MinStakePeriod:         defaultMinStakingDuration,
	MaxStakePeriod:         defaultMaxStakingDuration,
	StakePeriodRewardShare: 2_0000,
	StartRewardShare:       12_0000,
	StartRewardTime:        uint64(time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
	DiminishingRewardShare: 8_0000,
	DiminishingRewardTime:  uint64(time.Date(2029, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
	TargetRewardShare:      6_0000,
	TargetRewardTime:       uint64(time.Date(2030, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
}

func TestLongerDurationBonus(t *testing.T) {
	c := NewCalculator(defaultConfig)
	shortDuration := 24 * time.Hour
	totalDuration := 365 * 24 * time.Hour
	shortBalance := units.KiloAvax
	currentTime := time.Now()
	for i := 0; i < int(totalDuration/shortDuration); i++ {
		r := c.Calculate(shortDuration, currentTime, shortBalance)
		shortBalance += r
	}
	reward := c.Calculate(totalDuration%shortDuration, currentTime, shortBalance)
	shortBalance += reward

	longBalance := units.KiloAvax
	longBalance += c.Calculate(totalDuration, currentTime, longBalance)
	require.Less(t, shortBalance, longBalance, "should promote stakers to stake longer")
}

func TestRewards(t *testing.T) {
	c := NewCalculator(defaultConfig)
	startRewardTime := time.Unix(int64(defaultConfig.StartRewardTime), 0)
	diminishingRewardTime := time.Unix(int64(defaultConfig.DiminishingRewardTime), 0)
	targetRewardTime := time.Unix(int64(defaultConfig.TargetRewardTime), 0)
	tests := []struct {
		duration       time.Duration
		currentTime    time.Time
		stakeAmount    uint64
		expectedReward uint64
	}{
		// Max duration:
		{ // One day before start reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    startRewardTime.Add(time.Hour * -24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 140 * units.KiloAvax,
		},
		{ // At start reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    startRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 140 * units.KiloAvax,
		},
		{ // One day after start reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    startRewardTime.Add(time.Hour * 24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 139978 * units.Avax,
		},
		{ // One day before diminishing reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    diminishingRewardTime.Add(time.Hour * -24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 100021 * units.Avax,
		},
		{ // At diminishing reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    diminishingRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 100 * units.KiloAvax,
		},
		{ // One day after diminishing reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    diminishingRewardTime.Add(time.Hour * 24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 99945 * units.Avax,
		},
		{ // One day before target reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    targetRewardTime.Add(time.Hour * -24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 80054 * units.Avax,
		},
		{ // At target reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    targetRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 80 * units.KiloAvax,
		},
		{ // One day after target reward time
			duration:       defaultMaxStakingDuration,
			currentTime:    targetRewardTime.Add(time.Hour * 24),
			stakeAmount:    units.MegaAvax,
			expectedReward: 80 * units.KiloAvax,
		},
		// Min duration:
		{ // Four weeks before start reward time
			duration:       defaultMinStakingDuration,
			currentTime:    startRewardTime.Add(time.Hour * -24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 328 * units.Avax,
		},
		{ // At start reward time
			duration:       defaultMinStakingDuration,
			currentTime:    startRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 328 * units.Avax,
		},
		{ // Four weeks after start reward time
			duration:       defaultMinStakingDuration,
			currentTime:    startRewardTime.Add(time.Hour * 24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 326 * units.Avax,
		},
		{ // Four weeks before diminishing reward time
			duration:       defaultMinStakingDuration,
			currentTime:    diminishingRewardTime.Add(time.Hour * -24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 220 * units.Avax,
		},
		{ // At diminishing reward time
			duration:       defaultMinStakingDuration,
			currentTime:    diminishingRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 219 * units.Avax,
		},
		{ // Four weeks after diminishing reward time
			duration:       defaultMinStakingDuration,
			currentTime:    diminishingRewardTime.Add(time.Hour * 24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 214 * units.Avax,
		},
		{ // Four weeks before target reward time
			duration:       defaultMinStakingDuration,
			currentTime:    targetRewardTime.Add(time.Hour * -24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 168 * units.Avax,
		},
		{ // At target reward time
			duration:       defaultMinStakingDuration,
			currentTime:    targetRewardTime,
			stakeAmount:    units.MegaAvax,
			expectedReward: 164 * units.Avax,
		},
		{ // Four weeks after target reward time
			duration:       defaultMinStakingDuration,
			currentTime:    targetRewardTime.Add(time.Hour * 24 * 7 * 4),
			stakeAmount:    units.MegaAvax,
			expectedReward: 164 * units.Avax,
		},
	}
	for _, test := range tests {
		name := fmt.Sprintf("reward(%s,%s,%d)==%d",
			test.duration,
			test.currentTime,
			test.stakeAmount,
			test.expectedReward,
		)
		t.Run(name, func(t *testing.T) {
			reward := c.Calculate(
				test.duration,
				test.currentTime,
				test.stakeAmount,
			)
			require.Equal(t, test.expectedReward, reward)
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		amount        uint64
		shares        uint32
		expectedSplit uint64
	}{
		{
			amount:        1000,
			shares:        PercentDenominator / 2,
			expectedSplit: 500,
		},
		{
			amount:        1,
			shares:        PercentDenominator,
			expectedSplit: 1,
		},
		{
			amount:        1,
			shares:        PercentDenominator - 1,
			expectedSplit: 1,
		},
		{
			amount:        1,
			shares:        1,
			expectedSplit: 1,
		},
		{
			amount:        1,
			shares:        0,
			expectedSplit: 0,
		},
		{
			amount:        9223374036974675809,
			shares:        2,
			expectedSplit: 18446748749757,
		},
		{
			amount:        9223374036974675809,
			shares:        PercentDenominator,
			expectedSplit: 9223374036974675809,
		},
		{
			amount:        9223372036855275808,
			shares:        PercentDenominator - 2,
			expectedSplit: 9223353590111202098,
		},
		{
			amount:        9223372036855275808,
			shares:        2,
			expectedSplit: 18446744349518,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d_%d", test.amount, test.shares), func(t *testing.T) {
			require := require.New(t)

			split, remainder := Split(test.amount, test.shares)
			require.Equal(test.expectedSplit, split)
			require.Equal(test.amount-test.expectedSplit, remainder)
		})
	}
}
