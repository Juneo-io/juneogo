// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"
)

// PercentDenominator is the denominator used to calculate percentages
const PercentDenominator = 1_000_000

// Max bonus rewards given for a full staking period
const MaxBonusRewardShare = 20_000

// rewardShareDenominator is the magnitude offset used to emulate
// floating point fractions.
var rewardShareDenominator = new(big.Int).SetUint64(PercentDenominator)
var rewardShare = uint64(50_000)
var maxBonusRewardShare = new(big.Int).SetUint64(uint64(MaxBonusRewardShare))

type Config struct {
	// MaxConsumptionRate is the rate to allocate funds if the validator's stake
	// duration is equal to [MintingPeriod]
	MaxConsumptionRate uint64 `json:"maxConsumptionRate"`

	// MinConsumptionRate is the rate to allocate funds if the validator's stake
	// duration is 0.
	MinConsumptionRate uint64 `json:"minConsumptionRate"`

	// MintingPeriod is period that the staking calculator runs on. It is
	// not valid for a validator's stake duration to be larger than this.
	MintingPeriod time.Duration `json:"mintingPeriod"`

	// SupplyCap is the target value that the reward calculation should be
	// asymptotic to.
	SupplyCap uint64 `json:"supplyCap"`
}
