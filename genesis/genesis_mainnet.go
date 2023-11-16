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
	//go:embed genesis_mainnet.json
	mainnetGenesisConfigJSON []byte

	mainnetMinStakeDuration time.Duration = 2 * 7 * 24 * time.Hour
	mainnetMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// MainnetParams are the params used for mainnet
	MainnetParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         10 * units.MilliAvax,
			CreateAssetTxFee:              100 * units.MilliAvax,
			CreateSubnetTxFee:             100 * units.MilliAvax,
			TransformSubnetTxFee:          10 * units.Avax,
			CreateBlockchainTxFee:         100 * units.MilliAvax,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSubnetValidatorFee:         100 * units.MilliAvax,
			AddSubnetDelegatorFee:         100 * units.MilliAvax,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 1 * units.Avax,
			MaxValidatorStake: 1 * units.MegaAvax,
			MinDelegatorStake: 100 * units.MilliAvax,
			MinDelegationFee:  120000, // 12%
			MaxDelegationFee:  120000,
			MinStakeDuration:  mainnetMinStakeDuration,
			MaxStakeDuration:  mainnetMaxStakeDuration,
			RewardConfig: reward.Config{
				MinStakePeriod:         mainnetMinStakeDuration,
				MaxStakePeriod:         mainnetMaxStakeDuration,
				StakePeriodRewardShare: 2_0000,                                                             // 2%
				StartRewardShare:       21_5000,                                                            // 21.5%
				StartRewardTime:        uint64(time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC).Unix()), // 1st June 2023
				TargetRewardShare:      6_7000,                                                             // 6.7%
				TargetRewardTime:       uint64(time.Date(2028, time.June, 21, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)
