// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
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
			CreateSubnetTxFee:           100 * units.MilliAvax,
			TransformSubnetTxFee:        10 * units.Avax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSubnetValidatorFee:       100 * units.MilliAvax,
			AddSubnetDelegatorFee:       100 * units.MilliAvax,
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
				TargetRewardShare:      6_6000, // 6.6%
				TargetRewardTime:       uint64(time.Date(2030, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)
