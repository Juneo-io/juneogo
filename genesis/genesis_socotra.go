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

	// SocotraParams are the params used for the socotra testnet
	SocotraParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         10 * units.MilliAvax,
			CreateAssetTxFee:              100 * units.MilliAvax,
			CreateSupernetTxFee:           100 * units.MilliAvax,
			TransformSupernetTxFee:        1 * units.Avax,
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
			MinDelegatorStake: 10 * units.MilliAvax,
			MinDelegationFee:  120000, // 12%
			MinStakeDuration:  24 * time.Hour,
			MaxStakeDuration:  365 * 24 * time.Hour,
			RewardConfig: reward.Config{
				MintingPeriod: 365 * 24 * time.Hour,
				RewardShare:   65000, // 6.5%,
			},
		},
	}
)
