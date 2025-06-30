// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
)

var (
	//go:embed genesis_banana.json
	bananaGenesisConfigJSON []byte

	bananaMinStakeDuration time.Duration = 2 * 7 * 24 * time.Hour
	bananaMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// BananaParams are the params used for the banana testnet
	BananaParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         10 * units.MilliAvax,
			CreateAssetTxFee:              100 * units.MilliAvax,
			CreateSupernetTxFee:           100 * units.MilliAvax,
			TransformSupernetTxFee:        100 * units.Avax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSupernetValidatorFee:       100 * units.MilliAvax,
			AddSupernetDelegatorFee:       100 * units.MilliAvax,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 5 * units.Avax,
			MaxValidatorStake: 7500 * units.Avax,
			MinDelegatorStake: 100 * units.MilliAvax,
			MinDelegationFee:  50000, // 5%
			MaxDelegationFee:  50000,
			MinStakeDuration:  bananaMinStakeDuration,
			MaxStakeDuration:  bananaMaxStakeDuration,
			MaxValidatorWeightFactor: 10,
			RewardConfig: reward.Config{
				MinStakePeriod:         bananaMinStakeDuration,
				MaxStakePeriod:         bananaMaxStakeDuration,
				StakePeriodRewardShare: 2_0000,  // 2%
				StartRewardShare:       19_5000, // 19.5%
				StartRewardTime:        uint64(time.Date(2025, time.June, 15, 0, 0, 0, 0, time.UTC).Unix()),
				DiminishingRewardShare: 19_5000, // 19.5%
				DiminishingRewardTime:  uint64(time.Date(2030, time.June, 15, 0, 0, 0, 0, time.UTC).Unix()),
				TargetRewardShare:      6_6000, // 6.6%
				TargetRewardTime:       uint64(time.Date(2031, time.June, 15, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)
