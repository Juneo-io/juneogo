// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
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

// cushion valley very silver fragile car syrup slam army roast conduct jacket
// PrivateKey-VWEazFMoKnnFQuHyTsZKZwmTxj7ocC5mwCF7Wtm6XehLFy3E => P-local1f7z7rcerz4s5t4luth53tfystzly040phf94nm
// PrivateKey-VWEazFMoKnnFQuHyTsZKZwmTxj7ocC5mwCF7Wtm6XehLFy3E => JVM-local1f7z7rcerz4s5t4luth53tfystzly040phf94nm
// 4a51d8e8baff7fb8d08ea07ff1e8a62f60f490d46e4f917433cfd43cd280315e => 0xb4a56D9dBaB331eF6983dE1E7702d650D0154A53

const (
	VWEAKeyStr          = "VWEazFMoKnnFQuHyTsZKZwmTxj7ocC5mwCF7Wtm6XehLFy3E"
	VWEAKeyFormattedStr = secp256k1.PrivateKeyPrefix + VWEAKeyStr
)

var (
	VMRQKey *secp256k1.PrivateKey
	EWOQKey *secp256k1.PrivateKey

	//go:embed genesis_local.json
	localGenesisConfigJSON []byte

	// LocalParams are the params used for local networks
	LocalParams = Params{
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
				MaxConsumptionRate: .12 * reward.PercentDenominator,
				MinConsumptionRate: .10 * reward.PercentDenominator,
				MintingPeriod:      365 * 24 * time.Hour,
				SupplyCap:          720 * units.MegaAvax,
			},
		},
	}
)

func init() {
	errs := wrappers.Errs{}
	vmrqBytes, err := cb58.Decode(VWEAKeyStr)
	errs.Add(err)

	factory := secp256k1.Factory{}
	VMRQKey, err = factory.ToPrivateKey(vmrqBytes)
	errs.Add(err)

	if errs.Err != nil {
		panic(errs.Err)
	}
}
