// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/utils/units"
)

const (
	defaultMinStakingDuration = 24 * time.Hour
	defaultMaxStakingDuration = 365 * 24 * time.Hour

	defaultMinValidatorStake = 5 * units.MilliJune
)

var defaultConfig = Config{
	MintingPeriod: 365 * 24 * time.Hour,
	RewardShare:   50000,
}

func TestLongerDurationBonus(t *testing.T) {
	c := NewCalculator(defaultConfig)
	shortDuration := 24 * time.Hour
	totalDuration := 365 * 24 * time.Hour
	currentTime := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	shortBalance := units.KiloJune
	for i := 0; i < int(totalDuration/shortDuration); i++ {
		r := c.Calculate_(shortDuration, currentTime, shortBalance)
		shortBalance += r
	}
	r := c.Calculate_(totalDuration%shortDuration, currentTime, shortBalance)
	shortBalance += r

	longBalance := units.KiloJune
	longBalance += c.Calculate_(totalDuration, currentTime, longBalance)

	if shortBalance >= longBalance {
		t.Fatalf("should promote stakers to stake longer")
	}
}

func TestRewards(t *testing.T) {
	c := NewCalculator(defaultConfig)
	// TODO fix values to match juneo rewards
	tests := []struct {
		duration       time.Duration
		stakeAmount    uint64
		existingAmount uint64
		expectedReward uint64
	}{
		// Max duration:
		{ // (720M - 360M) * (1M / 360M) * 12%
			duration:       defaultMaxStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 360 * units.MegaJune,
			expectedReward: 120 * units.KiloJune,
		},
		{ // (720M - 400M) * (1M / 400M) * 12%
			duration:       defaultMaxStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 400 * units.MegaJune,
			expectedReward: 96 * units.KiloJune,
		},
		{ // (720M - 400M) * (2M / 400M) * 12%
			duration:       defaultMaxStakingDuration,
			stakeAmount:    2 * units.MegaJune,
			existingAmount: 400 * units.MegaJune,
			expectedReward: 192 * units.KiloJune,
		},
		{ // (720M - 720M) * (1M / 720M) * 12%
			duration:       defaultMaxStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 720 * units.MegaJune,
			expectedReward: 0,
		},
		// Min duration:
		// (720M - 360M) * (1M / 360M) * (10% + 2% * MinimumStakingDuration / MaximumStakingDuration) * MinimumStakingDuration / MaximumStakingDuration
		{
			duration:       defaultMinStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 360 * units.MegaJune,
			expectedReward: 274122724713,
		},
		// (720M - 360M) * (.005 / 360M) * (10% + 2% * MinimumStakingDuration / MaximumStakingDuration) * MinimumStakingDuration / MaximumStakingDuration
		{
			duration:       defaultMinStakingDuration,
			stakeAmount:    defaultMinValidatorStake,
			existingAmount: 360 * units.MegaJune,
			expectedReward: 1370,
		},
		// (720M - 400M) * (1M / 400M) * (10% + 2% * MinimumStakingDuration / MaximumStakingDuration) * MinimumStakingDuration / MaximumStakingDuration
		{
			duration:       defaultMinStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 400 * units.MegaJune,
			expectedReward: 219298179771,
		},
		// (720M - 400M) * (2M / 400M) * (10% + 2% * MinimumStakingDuration / MaximumStakingDuration) * MinimumStakingDuration / MaximumStakingDuration
		{
			duration:       defaultMinStakingDuration,
			stakeAmount:    2 * units.MegaJune,
			existingAmount: 400 * units.MegaJune,
			expectedReward: 438596359542,
		},
		// (720M - 720M) * (1M / 720M) * (10% + 2% * MinimumStakingDuration / MaximumStakingDuration) * MinimumStakingDuration / MaximumStakingDuration
		{
			duration:       defaultMinStakingDuration,
			stakeAmount:    units.MegaJune,
			existingAmount: 720 * units.MegaJune,
			expectedReward: 0,
		},
	}
	currentTime := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range tests {
		name := fmt.Sprintf("reward(%s,%d,%d)==%d",
			test.duration,
			test.stakeAmount,
			test.existingAmount,
			test.expectedReward,
		)
		t.Run(name, func(t *testing.T) {
			r := c.Calculate_(
				test.duration,
				currentTime,
				test.stakeAmount,
			)
			if r != test.expectedReward {
				t.Fatalf("expected %d; got %d", test.expectedReward, r)
			}
		})
	}
}

func TestRewardsOverflow(t *testing.T) {
	require := require.New(t)

	var (
		maxSupply     uint64 = math.MaxUint64
		initialSupply uint64 = 1
	)
	c := NewCalculator(Config{
		MintingPeriod: defaultMinStakingDuration,
		RewardShare:   50000,
	})
	currentTime := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	rewards := c.Calculate_(
		defaultMinStakingDuration,
		currentTime,
		maxSupply,
	)
	require.Equal(maxSupply-initialSupply, rewards)
}

func TestRewardsMint(t *testing.T) {
	require := require.New(t)

	var (
		maxSupply     uint64 = 1000
		initialSupply uint64 = 1
	)
	c := NewCalculator(Config{
		MintingPeriod: defaultMinStakingDuration,
		RewardShare:   50000,
	})
	currentTime := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	rewards := c.Calculate_(
		defaultMinStakingDuration,
		currentTime,
		maxSupply,
	)
	require.Equal(maxSupply-initialSupply, rewards)
}
