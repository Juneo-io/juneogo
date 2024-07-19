// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
)

var (
	_ UnsignedTx = (*TransformSupernetTx)(nil)

	errCantTransformPrimaryNetwork    = errors.New("cannot transform primary network")
	errEmptyAssetID                   = errors.New("empty asset ID is not valid")
	errAssetIDCantBeAVAX              = errors.New("asset ID can't be AVAX")
	errStartRewardShareZero           = errors.New("start reward share must be non-0")
	errStartRewardShareTooLarge       = fmt.Errorf("start reward share must be less than or equal to %d", reward.PercentDenominator)
	errStartRewardTimeZero            = errors.New("start reward time must be non-0")
	errStartRewardTimeTooLarge        = fmt.Errorf("start reward time must be less than or equal to diminishing reward time")
	errDiminishingRewardShareZero     = errors.New("diminishing reward share must be non-0")
	errDiminishingRewardShareTooLarge = fmt.Errorf("diminishing reward share must be less than or equal to start reward share")
	errDiminishingRewardTimeTooLarge  = fmt.Errorf("diminishing reward time must be less than or equal to target reward time")
	errTargetRewardShareZero          = errors.New("target reward share must be non-0")
	errTargetRewardShareTooLarge      = fmt.Errorf("target reward share must be less than or equal to diminishing reward share")
	errMinValidatorStakeZero          = errors.New("min validator stake must be non-0")
	errMinValidatorStakeAboveMax      = errors.New("min validator stake must be less than or equal to max validator stake")
	errMinStakeDurationZero           = errors.New("min stake duration must be non-0")
	errMinStakeDurationTooLarge       = errors.New("min stake duration must be less than or equal to max stake duration")
	errStakePeriodRewardShareZero     = errors.New("stake period reward share must be non-0")
	errStakePeriodRewardShareTooLarge = fmt.Errorf("stake period reward share must be less than or equal to %d", reward.PercentDenominator)
	errMinDelegationFeeTooLarge       = errors.New("min delegation fee must be less than or equal to MaxDelegationFee")
	errMaxDelegationFeeTooLarge       = fmt.Errorf("max delegation fee must be less than or equal to %d", reward.PercentDenominator)
	errMinDelegatorStakeZero          = errors.New("min delegator stake must be non-0")
	errMaxValidatorWeightFactorZero   = errors.New("max validator weight factor must be non-0")
	errUptimeRequirementTooLarge      = fmt.Errorf("uptime requirement must be less than or equal to %d", reward.PercentDenominator)
)

// TransformSupernetTx is an unsigned transformSupernetTx
type TransformSupernetTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// ID of the Supernet to transform
	// Restrictions:
	// - Must not be the Primary Network ID
	Supernet ids.ID `serialize:"true" json:"supernetID"`
	// Asset to use when staking on the Supernet
	// Restrictions:
	// - Must not be the Empty ID
	// - Must not be the AVAX ID
	AssetID ids.ID `serialize:"true" json:"assetID"`
	// Amount to specify as the amount of rewards that will be initially
	// available in the reward pool of the supernet.
	InitialRewardPoolSupply uint64 `serialize:"true" json:"initialRewardPoolSupply"`
	// StartRewardShare is the starting share of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [reward.PercentDenominator]
	StartRewardShare uint64 `serialize:"true" json:"startRewardShare"`
	// StartRewardTime is the starting timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [TargetRewardTime]
	StartRewardTime uint64 `serialize:"true" json:"startRewardTime"`
	// DiminishingRewardShare is the share of rewards given to validators at the start of diminishing year.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [StartRewardShare]
	DiminishingRewardShare uint64 `serialize:"true" json:"diminishingRewardShare"`
	// DiminishingRewardTime is the target timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Restrictions:
	// - Must be >= [StartRewardTime]
	DiminishingRewardTime uint64 `serialize:"true" json:"diminishingRewardTime"`
	// TargetRewardShare is the target final share of rewards given to validators.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [DiminishingRewardShare]
	TargetRewardShare uint64 `serialize:"true" json:"targetRewardShare"`
	// TargetRewardTime is the target timestamp that will be used to calculate
	// the remaining percentage of rewards given to validators.
	// Restrictions:
	// - Must be >= [DiminishingRewardTime]
	TargetRewardTime uint64 `serialize:"true" json:"targetRewardTime"`
	// MinValidatorStake is the minimum amount of funds required to become a
	// validator.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [InitialSupply]
	MinValidatorStake uint64 `serialize:"true" json:"minValidatorStake"`
	// MaxValidatorStake is the maximum amount of funds a single validator can
	// be allocated, including delegated funds.
	// Restrictions:
	// - Must be >= [MinValidatorStake]
	// - Must be <= [MaximumSupply]
	MaxValidatorStake uint64 `serialize:"true" json:"maxValidatorStake"`
	// MinStakeDuration is the minimum number of seconds a staker can stake for.
	// Restrictions:
	// - Must be > 0
	MinStakeDuration uint32 `serialize:"true" json:"minStakeDuration"`
	// MaxStakeDuration is the maximum number of seconds a staker can stake for.
	// Restrictions:
	// - Must be >= [MinStakeDuration]
	// - Must be <= [GlobalMaxStakeDuration]
	MaxStakeDuration uint32 `serialize:"true" json:"maxStakeDuration"`
	// StakePeriodRewardShare is the maximum period reward given for a
	// stake period equal to MaxStakePeriod.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [reward.PercentDenominator]
	StakePeriodRewardShare uint64 `serialize:"true" json:"stakePeriodRewardShare"`
	// MinDelegationFee is the minimum percentage a validator must charge a
	// delegator for delegating.
	// Restrictions:
	// - Must be <= [MaxDelegationFee]
	MinDelegationFee uint32 `serialize:"true" json:"minDelegationFee"`
	// MaxDelegationFee is the minimum percentage a validator must charge a
	// delegator for delegating.
	// Restrictions:
	// - Must be <= [reward.PercentDenominator]
	MaxDelegationFee uint32 `serialize:"true" json:"maxDelegationFee"`
	// MinDelegatorStake is the minimum amount of funds required to become a
	// delegator.
	// Restrictions:
	// - Must be > 0
	MinDelegatorStake uint64 `serialize:"true" json:"minDelegatorStake"`
	// MaxValidatorWeightFactor is the factor which calculates the maximum
	// amount of delegation a validator can receive.
	// Note: a value of 1 effectively disables delegation.
	// Restrictions:
	// - Must be > 0
	MaxValidatorWeightFactor byte `serialize:"true" json:"maxValidatorWeightFactor"`
	// UptimeRequirement is the minimum percentage a validator must be online
	// and responsive to receive a reward.
	// Restrictions:
	// - Must be <= [reward.PercentDenominator]
	UptimeRequirement uint32 `serialize:"true" json:"uptimeRequirement"`
	// Authorizes this transformation
	SupernetAuth verify.Verifiable `serialize:"true" json:"supernetAuthorization"`
}

func (tx *TransformSupernetTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.Supernet == constants.PrimaryNetworkID:
		return errCantTransformPrimaryNetwork
	case tx.AssetID == ids.Empty:
		return errEmptyAssetID
	case tx.AssetID == ctx.JUNEAssetID:
		return errAssetIDCantBeAVAX
	case tx.StartRewardShare == 0:
		return errStartRewardShareZero
	case tx.StartRewardShare > reward.PercentDenominator:
		return errStartRewardShareTooLarge
	case tx.StartRewardTime == 0:
		return errStartRewardTimeZero
	case tx.StartRewardTime > tx.DiminishingRewardTime:
		return errStartRewardTimeTooLarge
	case tx.DiminishingRewardShare == 0:
		return errDiminishingRewardShareZero
	case tx.DiminishingRewardShare > tx.StartRewardShare:
		return errDiminishingRewardShareTooLarge
	case tx.DiminishingRewardTime > tx.TargetRewardTime:
		return errDiminishingRewardTimeTooLarge
	case tx.TargetRewardShare == 0:
		return errTargetRewardShareZero
	case tx.TargetRewardShare > tx.DiminishingRewardShare:
		return errTargetRewardShareTooLarge
	case tx.MinValidatorStake == 0:
		return errMinValidatorStakeZero
	case tx.MinValidatorStake > tx.MaxValidatorStake:
		return errMinValidatorStakeAboveMax
	case tx.MinStakeDuration == 0:
		return errMinStakeDurationZero
	case tx.MinStakeDuration > tx.MaxStakeDuration:
		return errMinStakeDurationTooLarge
	case tx.StakePeriodRewardShare == 0:
		return errStakePeriodRewardShareZero
	case tx.StakePeriodRewardShare > reward.PercentDenominator:
		return errStakePeriodRewardShareTooLarge
	case tx.MinDelegationFee > tx.MaxDelegationFee:
		return errMinDelegationFeeTooLarge
	case tx.MaxDelegationFee > reward.PercentDenominator:
		return errMaxDelegationFeeTooLarge
	case tx.MinDelegatorStake == 0:
		return errMinDelegatorStakeZero
	case tx.MaxValidatorWeightFactor == 0:
		return errMaxValidatorWeightFactorZero
	case tx.UptimeRequirement > reward.PercentDenominator:
		return errUptimeRequirementTooLarge
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := tx.SupernetAuth.Verify(); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *TransformSupernetTx) Visit(visitor Visitor) error {
	return visitor.TransformSupernetTx(tx)
}
