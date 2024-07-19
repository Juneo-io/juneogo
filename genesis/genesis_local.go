// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/Juneo-io/juneogo/utils/cb58"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
)

// PrivateKey-vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE => P-local1g65uqn6t77p656w64023nh8nd9updzmxyymev2
// PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN => X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u
// 56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 => 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC

const (
	VMRQKeyStr          = "vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE"
	VMRQKeyFormattedStr = secp256k1.PrivateKeyPrefix + VMRQKeyStr

	EWOQKeyStr          = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	EWOQKeyFormattedStr = secp256k1.PrivateKeyPrefix + EWOQKeyStr
)

var (
	VMRQKey *secp256k1.PrivateKey
	EWOQKey *secp256k1.PrivateKey

	//go:embed genesis_local.json
	localGenesisConfigJSON []byte

	localMinStakeDuration time.Duration = 24 * time.Hour
	localMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// LocalParams are the params used for local networks
	LocalParams = Params{
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
			MaxDelegationFee:  200000, // 20%
			MinStakeDuration:  localMinStakeDuration,
			MaxStakeDuration:  localMaxStakeDuration,
			RewardConfig: reward.Config{
				MinStakePeriod:         localMinStakeDuration,
				MaxStakePeriod:         localMaxStakeDuration,
				StakePeriodRewardShare: 2_0000,                                                             // 2%
				StartRewardShare:       21_5000,                                                            // 21.5%
				StartRewardTime:        uint64(time.Date(2023, time.June, 1, 0, 0, 0, 0, time.UTC).Unix()), // 1st June 2023
				DiminishingRewardTime:  uint64(time.Date(2027, time.June, 21, 0, 0, 0, 0, time.UTC).Unix()),
				DiminishingRewardShare: uint64(19_5000),
				TargetRewardShare:      6_7000, // 6.7%
				TargetRewardTime:       uint64(time.Date(2028, time.June, 21, 0, 0, 0, 0, time.UTC).Unix()),
			},
		},
	}
)

func init() {
	errs := wrappers.Errs{}
	vmrqBytes, err := cb58.Decode(VMRQKeyStr)
	errs.Add(err)
	ewoqBytes, err := cb58.Decode(EWOQKeyStr)
	errs.Add(err)

	VMRQKey, err = secp256k1.ToPrivateKey(vmrqBytes)
	errs.Add(err)
	EWOQKey, err = secp256k1.ToPrivateKey(ewoqBytes)
	errs.Add(err)

	if errs.Err != nil {
		panic(errs.Err)
	}
}
