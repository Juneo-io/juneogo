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
	//go:embed genesis_fuji.json
	fujiGenesisConfigJSON []byte

	// FujiParams are the params used for the fuji testnet
	FujiParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         units.MilliJune,
			CreateAssetTxFee:              10 * units.MilliJune,
			CreateSupernetTxFee:           100 * units.MilliJune,
			TransformSupernetTxFee:        1 * units.June,
			CreateBlockchainTxFee:         100 * units.MilliJune,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSupernetValidatorFee:       units.MilliJune,
			AddSupernetDelegatorFee:       units.MilliJune,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 1 * units.June,
			MaxValidatorStake: 3 * units.MegaJune,
			MinDelegatorStake: 1 * units.June,
			MinDelegationFee:  20000, // 2%
			MinStakeDuration:  24 * time.Hour,
			MaxStakeDuration:  365 * 24 * time.Hour,
			RewardConfig: reward.Config{
				MintingPeriod: 365 * 24 * time.Hour,
				RewardShare:   50000, // 5%
			},
		},
	}
)
