// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	_ "embed"

	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
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

	localMinStakeDuration time.Duration = 24 * time.Hour
	localMaxStakeDuration time.Duration = 365 * 24 * time.Hour

	// LocalParams are the params used for local networks
	LocalParams = Params{
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
	vmrqBytes, err := cb58.Decode(VWEAKeyStr)
	errs.Add(err)

	VMRQKey, err = secp256k1.ToPrivateKey(vmrqBytes)
	errs.Add(err)

	if errs.Err != nil {
		panic(errs.Err)
	}
}
