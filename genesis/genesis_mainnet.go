// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
)

var (
	//go:embed genesis_mainnet.json
	mainnetGenesisConfigJSON []byte

	mainnetMinStakeDuration time.Duration = 2 * 7 * 24 * time.Hour
	mainnetMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// MainnetParams are the params used for mainnet
	MainnetParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         3 * units.MilliAvax,
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
			MinValidatorStake: 100 * units.Avax,
			MaxValidatorStake: 30 * units.KiloAvax,
			MinDelegatorStake: 10 * units.MilliAvax,
			MinDelegationFee:  120000, // 12%
			MaxDelegationFee:  120000,
			MinStakeDuration:  mainnetMinStakeDuration,
			MaxStakeDuration:  mainnetMaxStakeDuration,
			RewardConfig: reward.Config{
				MinStakePeriod:         mainnetMinStakeDuration,
				MaxStakePeriod:         mainnetMaxStakeDuration,
				StakePeriodRewardShare: 2_0000,  // 2%
				StartRewardShare:       21_5000, // 21.5%
				StartRewardTime:        uint64(time.Date(2024, time.July, 15, 0, 0, 0, 0, time.UTC).Unix()),
				DiminishingRewardShare: 19_0000, // 19%
				DiminishingRewardTime:  uint64(time.Date(2029, time.July, 15, 0, 0, 0, 0, time.UTC).Unix()),
				TargetRewardShare:      6_6000, // 6.6%
				TargetRewardTime:       uint64(time.Date(2030, time.July, 15, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)
