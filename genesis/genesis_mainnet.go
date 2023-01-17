// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
)

var (
	//go:embed genesis_mainnet.json
	mainnetGenesisConfigJSON []byte

	// MainnetParams are the params used for mainnet
	MainnetParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         units.MilliJune,
			CreateAssetTxFee:              10 * units.MilliJune,
			CreateSupernetTxFee:           1 * units.June,
			TransformSupernetTxFee:        10 * units.June,
			CreateBlockchainTxFee:         1 * units.June,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSupernetValidatorFee:       units.MilliJune,
			AddSupernetDelegatorFee:       units.MilliJune,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 100 * units.June,
			MaxValidatorStake: 45 * units.KiloJune,
			MinDelegatorStake: 10 * units.MilliJune,
			MinDelegationFee:  120000, // 12%
			MinStakeDuration:  2 * 7 * 24 * time.Hour,
			MaxStakeDuration:  365 * 24 * time.Hour,
			RewardConfig: reward.Config{
				MintingPeriod: 365 * 24 * time.Hour,
				RewardShare:   50000, // 5%
			},
		},
	}
)
