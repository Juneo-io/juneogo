// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"
)

// PercentDenominator is the denominator used to calculate percentages
const PercentDenominator = 1_000_000

// rewardShareDenominator is the magnitude offset used to emulate
// floating point fractions.
var rewardShareDenominator = new(big.Int).SetUint64(PercentDenominator)

type Config struct {
	// MinStakePeriod is the minimal stake duration.
	MinStakePeriod time.Duration
	// MaxStakePeriod is the maximum stake duration.
	MaxStakePeriod time.Duration
	// StakePeriodRewardShare is the maximum period reward given for a
	// stake period equal to MaxStakePeriod.
	StakePeriodRewardShare uint64
	// StartRewardShare is the starting share of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [reward.PercentDenominator]
	StartRewardShare uint64 `serialize:"true" json:"startRewardShare"`
	// StartRewardTime is the starting timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [TargetRewardTime]
	StartRewardTime uint64 `serialize:"true" json:"startRewardTime"`
	// DiminishingRewardShare is the share of rewards given to validators at the start of diminishing year.
	// Used only for calculation on Primary.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [StartRewardShare]
	DiminishingRewardShare uint64 `serialize:"true" json:"diminishingRewardShare"`
	// DiminishingRewardTime is the target timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Used only for calculation on Primary.
	// Restrictions:
	// - Must be >= [StartRewardTime]
	DiminishingRewardTime uint64 `serialize:"true" json:"diminishingRewardTime"`
	// TargetRewardShare is the target final share of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [DiminishingRewardShare]
	TargetRewardShare uint64 `serialize:"true" json:"targetRewardShare"`
	// TargetRewardTime is the target timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Restrictions:
	// - Must be >= [DiminishingRewardTime]
	TargetRewardTime uint64 `serialize:"true" json:"targetRewardTime"`
}
