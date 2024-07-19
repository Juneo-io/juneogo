// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

var _ Builder = (*builderWithOptions)(nil)

type builderWithOptions struct {
	builder Builder
	options []common.Option
}

// NewWithOptions returns a new builder that will use the given options by
// default.
//
//   - [builder] is the builder that will be called to perform the underlying
//     operations.
//   - [options] will be provided to the builder in addition to the options
//     provided in the method calls.
func NewWithOptions(builder Builder, options ...common.Option) Builder {
	return &builderWithOptions{
		builder: builder,
		options: options,
	}
}

func (b *builderWithOptions) Context() *Context {
	return b.builder.Context()
}

func (b *builderWithOptions) GetBalance(
	options ...common.Option,
) (map[ids.ID]uint64, error) {
	return b.builder.GetBalance(
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) GetImportableBalance(
	chainID ids.ID,
	options ...common.Option,
) (map[ids.ID]uint64, error) {
	return b.builder.GetImportableBalance(
		chainID,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewBaseTx(
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.BaseTx, error) {
	return b.builder.NewBaseTx(
		outputs,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewAddValidatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	options ...common.Option,
) (*txs.AddValidatorTx, error) {
	return b.builder.NewAddValidatorTx(
		vdr,
		rewardsOwner,
		shares,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewAddSupernetValidatorTx(
	vdr *txs.SupernetValidator,
	options ...common.Option,
) (*txs.AddSupernetValidatorTx, error) {
	return b.builder.NewAddSupernetValidatorTx(
		vdr,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewRemoveSupernetValidatorTx(
	nodeID ids.NodeID,
	supernetID ids.ID,
	options ...common.Option,
) (*txs.RemoveSupernetValidatorTx, error) {
	return b.builder.NewRemoveSupernetValidatorTx(
		nodeID,
		supernetID,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewAddDelegatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.AddDelegatorTx, error) {
	return b.builder.NewAddDelegatorTx(
		vdr,
		rewardsOwner,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewCreateChainTx(
	supernetID ids.ID,
	genesis []byte,
	vmID ids.ID,
	fxIDs []ids.ID,
	chainName string,
	chainAssetID ids.ID,
	options ...common.Option,
) (*txs.CreateChainTx, error) {
	return b.builder.NewCreateChainTx(
		supernetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		chainAssetID,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewCreateSupernetTx(
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.CreateSupernetTx, error) {
	return b.builder.NewCreateSupernetTx(
		owner,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewTransferSupernetOwnershipTx(
	supernetID ids.ID,
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.TransferSupernetOwnershipTx, error) {
	return b.builder.NewTransferSupernetOwnershipTx(
		supernetID,
		owner,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewImportTx(
	sourceChainID ids.ID,
	to *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.ImportTx, error) {
	return b.builder.NewImportTx(
		sourceChainID,
		to,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewExportTx(
	chainID ids.ID,
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.ExportTx, error) {
	return b.builder.NewExportTx(
		chainID,
		outputs,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewTransformSupernetTx(
	supernetID ids.ID,
	assetID ids.ID,
	initialRewardPoolSupply uint64,
	startRewardShare uint64,
	startRewardTime uint64,
	diminishingRewardShare uint64,
	diminishingRewardTime uint64,
	targetRewardShare uint64,
	targetRewardTime uint64,
	minValidatorStake uint64,
	maxValidatorStake uint64,
	minStakeDuration time.Duration,
	maxStakeDuration time.Duration,
	stakePeriodRewardShare uint64,
	minDelegationFee uint32,
	maxDelegationFee uint32,
	minDelegatorStake uint64,
	maxValidatorWeightFactor byte,
	uptimeRequirement uint32,
	options ...common.Option,
) (*txs.TransformSupernetTx, error) {
	return b.builder.NewTransformSupernetTx(
		supernetID,
		assetID,
		initialRewardPoolSupply,
		startRewardShare,
		startRewardTime,
		diminishingRewardShare,
		diminishingRewardTime,
		targetRewardShare,
		targetRewardTime,
		minValidatorStake,
		maxValidatorStake,
		minStakeDuration,
		maxStakeDuration,
		stakePeriodRewardShare,
		minDelegationFee,
		maxDelegationFee,
		minDelegatorStake,
		maxValidatorWeightFactor,
		uptimeRequirement,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewAddPermissionlessValidatorTx(
	vdr *txs.SupernetValidator,
	signer signer.Signer,
	assetID ids.ID,
	validationRewardsOwner *secp256k1fx.OutputOwners,
	delegationRewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	options ...common.Option,
) (*txs.AddPermissionlessValidatorTx, error) {
	return b.builder.NewAddPermissionlessValidatorTx(
		vdr,
		signer,
		assetID,
		validationRewardsOwner,
		delegationRewardsOwner,
		shares,
		common.UnionOptions(b.options, options)...,
	)
}

func (b *builderWithOptions) NewAddPermissionlessDelegatorTx(
	vdr *txs.SupernetValidator,
	assetID ids.ID,
	rewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.AddPermissionlessDelegatorTx, error) {
	return b.builder.NewAddPermissionlessDelegatorTx(
		vdr,
		assetID,
		rewardsOwner,
		common.UnionOptions(b.options, options)...,
	)
}
