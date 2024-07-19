// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	_ ValidatorTx     = (*AddValidatorTx)(nil)
	_ ScheduledStaker = (*AddValidatorTx)(nil)

	errTooManyShares = fmt.Errorf("a staker can only require at most %d shares from delegators", reward.PercentDenominator)
)

// AddValidatorTx is an unsigned addValidatorTx
type AddValidatorTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// Describes the delegatee
	Validator `serialize:"true" json:"validator"`
	// Where to send staked tokens when done validating
	StakeOuts []*avax.TransferableOutput `serialize:"true" json:"stake"`
	// Where to send staking rewards when done validating
	RewardsOwner fx.Owner `serialize:"true" json:"rewardsOwner"`
	// Fee this validator charges delegators as a percentage, times 10,000
	// For example, if this validator has DelegationShares=300,000 then they
	// take 30% of rewards from delegators
	DelegationShares uint32 `serialize:"true" json:"shares"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [AddValidatorTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *AddValidatorTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, out := range tx.StakeOuts {
		out.FxID = secp256k1fx.ID
		out.InitCtx(ctx)
	}
	tx.RewardsOwner.InitCtx(ctx)
}

func (tx *AddValidatorTx) ConsumedValue(assetID ids.ID) uint64 {
	value := tx.BaseTx.ConsumedValue(assetID)
	for _, out := range tx.StakeOuts {
		if out.Asset.AssetID() == assetID {
			val, err := math.Sub(value, out.Out.Amount())
			if err != nil {
				return uint64(0)
			}
			value = val
		}
	}
	return value
}

func (*AddValidatorTx) SupernetID() ids.ID {
	return constants.PrimaryNetworkID
}

func (tx *AddValidatorTx) NodeID() ids.NodeID {
	return tx.Validator.NodeID
}

func (*AddValidatorTx) PublicKey() (*bls.PublicKey, bool, error) {
	return nil, false, nil
}

func (*AddValidatorTx) PendingPriority() Priority {
	return PrimaryNetworkValidatorPendingPriority
}

func (*AddValidatorTx) CurrentPriority() Priority {
	return PrimaryNetworkValidatorCurrentPriority
}

func (tx *AddValidatorTx) Stake() []*avax.TransferableOutput {
	return tx.StakeOuts
}

func (tx *AddValidatorTx) ValidationRewardsOwner() fx.Owner {
	return tx.RewardsOwner
}

func (tx *AddValidatorTx) DelegationRewardsOwner() fx.Owner {
	return tx.RewardsOwner
}

func (tx *AddValidatorTx) Shares() uint32 {
	return tx.DelegationShares
}

// SyntacticVerify returns nil iff [tx] is valid
func (tx *AddValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.DelegationShares > reward.PercentDenominator: // Ensure delegators shares are in the allowed amount
		return errTooManyShares
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}
	if err := verify.All(&tx.Validator, tx.RewardsOwner); err != nil {
		return fmt.Errorf("failed to verify validator or rewards owner: %w", err)
	}

	totalStakeWeight := uint64(0)
	for _, out := range tx.StakeOuts {
		if err := out.Verify(); err != nil {
			return fmt.Errorf("failed to verify output: %w", err)
		}
		newWeight, err := math.Add64(totalStakeWeight, out.Output().Amount())
		if err != nil {
			return err
		}
		totalStakeWeight = newWeight

		assetID := out.AssetID()
		if assetID != ctx.JUNEAssetID {
			return fmt.Errorf("%w but is %q", errStakeMustBeAVAX, assetID)
		}
	}

	switch {
	case !avax.IsSortedTransferableOutputs(tx.StakeOuts, Codec):
		return errOutputsNotSorted
	case totalStakeWeight != tx.Wght:
		return fmt.Errorf("%w: weight %d != stake %d", errValidatorWeightMismatch, tx.Wght, totalStakeWeight)
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *AddValidatorTx) Visit(visitor Visitor) error {
	return visitor.AddValidatorTx(tx)
}
