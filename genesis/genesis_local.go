// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/Juneo-io/juneogo/utils/cb58"
	"github.com/Juneo-io/juneogo/utils/crypto"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
)

// PrivateKey-vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE => P-local1g65uqn6t77p656w64023nh8nd9updzmxyymev2
// PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN => X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u
// 56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 => 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC

const (
	VMRQKeyStr          = "vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE"
	VMRQKeyFormattedStr = crypto.PrivateKeyPrefix + VMRQKeyStr

	EWOQKeyStr          = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	EWOQKeyFormattedStr = crypto.PrivateKeyPrefix + EWOQKeyStr
)

var (
	VMRQKey *crypto.PrivateKeySECP256K1R
	EWOQKey *crypto.PrivateKeySECP256K1R

	//go:embed genesis_local.json
	localGenesisConfigJSON []byte

	// LocalParams are the params used for local networks
	LocalParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                         units.MilliJune,
			CreateAssetTxFee:              units.MilliJune,
			CreateSupernetTxFee:           100 * units.MilliJune,
			TransformSupernetTxFee:        100 * units.MilliJune,
			CreateBlockchainTxFee:         100 * units.MilliJune,
			AddPrimaryNetworkValidatorFee: 0,
			AddPrimaryNetworkDelegatorFee: 0,
			AddSupernetValidatorFee:       units.MilliJune,
			AddSupernetDelegatorFee:       units.MilliJune,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinValidatorStake: 2 * units.KiloJune,
			MaxValidatorStake: 3 * units.MegaJune,
			MinDelegatorStake: 25 * units.June,
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

func init() {
	errs := wrappers.Errs{}
	vmrqBytes, err := cb58.Decode(VMRQKeyStr)
	errs.Add(err)
	ewoqBytes, err := cb58.Decode(EWOQKeyStr)
	errs.Add(err)

	factory := crypto.FactorySECP256K1R{}
	vmrqIntf, err := factory.ToPrivateKey(vmrqBytes)
	errs.Add(err)
	ewoqIntf, err := factory.ToPrivateKey(ewoqBytes)
	errs.Add(err)

	if errs.Err != nil {
		panic(errs.Err)
	}

	VMRQKey = vmrqIntf.(*crypto.PrivateKeySECP256K1R)
	EWOQKey = ewoqIntf.(*crypto.PrivateKeySECP256K1R)
}
