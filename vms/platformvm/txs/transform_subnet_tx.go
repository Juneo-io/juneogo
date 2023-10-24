// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
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

	errCantTransformPrimaryNetwork  = errors.New("cannot transform primary network")
	errEmptyAssetID                 = errors.New("empty asset ID is not valid")
	errAssetIDCantBeAVAX            = errors.New("asset ID can't be AVAX")
	errInitialRewardsPoolSupplyZero = errors.New("initial rewards pool supply must be non-0")
	errRewardShareZero              = errors.New("reward share must be non-0")
	errRewardShareTooLarge          = fmt.Errorf("reward share must be less than or equal to %d", reward.PercentDenominator)
	errMinValidatorStakeZero        = errors.New("min validator stake must be non-0")
	errMinValidatorStakeAboveMax    = errors.New("min validator stake must be less than or equal to max validator stake")
	errMinStakeDurationZero         = errors.New("min stake duration must be non-0")
	errMinStakeDurationTooLarge     = errors.New("min stake duration must be less than or equal to max stake duration")
	errMinDelegationFeeTooLarge     = fmt.Errorf("min delegation fee must be less than or equal to %d", reward.PercentDenominator)
	errMinDelegatorStakeZero        = errors.New("min delegator stake must be non-0")
	errMaxValidatorWeightFactorZero = errors.New("max validator weight factor must be non-0")
	errUptimeRequirementTooLarge    = fmt.Errorf("uptime requirement must be less than or equal to %d", reward.PercentDenominator)
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
	// available in the rewards pool of the subnet.
	// Restrictions:
	// - Must be > 0
	InitialRewardsPoolSupply uint64 `serialize:"true" json:"initialRewardsPoolSupply"`
	// RewardShare is the share of rewards given for validators.
	// Restrictions:
	// - Must be > 0
	// - Must be < [reward.PercentDenominator]
	RewardShare uint64 `serialize:"true" json:"rewardShare"`
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
	// MinDelegationFee is the minimum percentage a validator must charge a
	// delegator for delegating.
	// Restrictions:
	// - Must be <= [reward.PercentDenominator]
	MinDelegationFee uint32 `serialize:"true" json:"minDelegationFee"`
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
	case tx.AssetID == ctx.AVAXAssetID:
		return errAssetIDCantBeAVAX
	case tx.InitialRewardsPoolSupply == 0:
		return errInitialRewardsPoolSupplyZero
	case tx.RewardShare == 0:
		return errRewardShareZero
	case tx.RewardShare > reward.PercentDenominator:
		return errRewardShareTooLarge
	case tx.MinValidatorStake == 0:
		return errMinValidatorStakeZero
	case tx.MinValidatorStake > tx.MaxValidatorStake:
		return errMinValidatorStakeAboveMax
	case tx.MinStakeDuration == 0:
		return errMinStakeDurationZero
	case tx.MinStakeDuration > tx.MaxStakeDuration:
		return errMinStakeDurationTooLarge
	case tx.MinDelegationFee > reward.PercentDenominator:
		return errMinDelegationFeeTooLarge
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
