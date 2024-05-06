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
	//go:embed genesis_socotra.json
	socotraGenesisConfigJSON []byte

	socotraMinStakeDuration time.Duration = 2 * 7 * 24 * time.Hour
	socotraMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// SocotraParams are the params used for the socotra testnet
	SocotraParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         10 * units.MilliAvax,
			CreateAssetTxFee:              100 * units.MilliAvax,
			CreateSupernetTxFee:           100 * units.MilliAvax,
			TransformSupernetTxFee:        10 * units.Avax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSupernetValidatorFee:       100 * units.MilliAvax,
			AddSupernetDelegatorFee:       100 * units.MilliAvax,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 1 * units.Avax,
			MaxValidatorStake: 1 * units.MegaAvax,
			MinDelegatorStake: 100 * units.MilliAvax,
			MinDelegationFee:  120000, // 12%
			MaxDelegationFee:  120000,
			MinStakeDuration:  socotraMinStakeDuration,
			MaxStakeDuration:  socotraMaxStakeDuration,
			RewardConfig: reward.Config{
				MinStakePeriod:         socotraMinStakeDuration,
				MaxStakePeriod:         socotraMaxStakeDuration,
				StakePeriodRewardShare: 2_0000,  // 2%
				StartRewardShare:       21_5000, // 21.5%
				StartRewardTime:        uint64(time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
				DiminishingRewardShare: 19_0000, // 19%
				DiminishingRewardTime:  uint64(time.Date(2029, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
				TargetRewardShare:      6_8000, // 6.8%
				TargetRewardTime:       uint64(time.Date(2030, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)
