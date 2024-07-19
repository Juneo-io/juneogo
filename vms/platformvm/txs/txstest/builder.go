// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txstest

import (
	"context"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/config"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/chain/p/builder"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"

	vmsigner "github.com/Juneo-io/juneogo/vms/platformvm/signer"
	walletsigner "github.com/Juneo-io/juneogo/wallet/chain/p/signer"
)

func NewBuilder(
	ctx *snow.Context,
	cfg *config.Config,
	state state.State,
) *Builder {
	return &Builder{
		ctx:   ctx,
		cfg:   cfg,
		state: state,
	}
}

type Builder struct {
	ctx   *snow.Context
	cfg   *config.Config
	state state.State
}

func (b *Builder) NewImportTx(
	chainID ids.ID,
	to *secp256k1fx.OutputOwners,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewImportTx(
		chainID,
		to,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building import tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewExportTx(
	chainID ids.ID,
	outputs []*avax.TransferableOutput,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewExportTx(
		chainID,
		outputs,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building export tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewCreateChainTx(
	supernetID ids.ID,
	genesis []byte,
	vmID ids.ID,
	fxIDs []ids.ID,
	chainName string,
	chainAssetID ids.ID,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewCreateChainTx(
		supernetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		chainAssetID,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building create chain tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewCreateSupernetTx(
	owner *secp256k1fx.OutputOwners,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewCreateSupernetTx(
		owner,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building create supernet tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewTransformSupernetTx(
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
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewTransformSupernetTx(
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
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building transform supernet tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewAddValidatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewAddValidatorTx(
		vdr,
		rewardsOwner,
		shares,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building add validator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewAddPermissionlessValidatorTx(
	vdr *txs.SupernetValidator,
	signer vmsigner.Signer,
	assetID ids.ID,
	validationRewardsOwner *secp256k1fx.OutputOwners,
	delegationRewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewAddPermissionlessValidatorTx(
		vdr,
		signer,
		assetID,
		validationRewardsOwner,
		delegationRewardsOwner,
		shares,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building add permissionless validator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewAddDelegatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewAddDelegatorTx(
		vdr,
		rewardsOwner,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building add delegator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewAddPermissionlessDelegatorTx(
	vdr *txs.SupernetValidator,
	assetID ids.ID,
	rewardsOwner *secp256k1fx.OutputOwners,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewAddPermissionlessDelegatorTx(
		vdr,
		assetID,
		rewardsOwner,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building add permissionless delegator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewAddSupernetValidatorTx(
	vdr *txs.SupernetValidator,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewAddSupernetValidatorTx(
		vdr,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building add supernet validator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewRemoveSupernetValidatorTx(
	nodeID ids.NodeID,
	supernetID ids.ID,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewRemoveSupernetValidatorTx(
		nodeID,
		supernetID,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building remove supernet validator tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewTransferSupernetOwnershipTx(
	supernetID ids.ID,
	owner *secp256k1fx.OutputOwners,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewTransferSupernetOwnershipTx(
		supernetID,
		owner,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building transfer supernet ownership tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) NewBaseTx(
	outputs []*avax.TransferableOutput,
	keys []*secp256k1.PrivateKey,
	options ...common.Option,
) (*txs.Tx, error) {
	pBuilder, pSigner := b.builders(keys)

	utx, err := pBuilder.NewBaseTx(
		outputs,
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed building base tx: %w", err)
	}

	return walletsigner.SignUnsigned(context.Background(), pSigner, utx)
}

func (b *Builder) builders(keys []*secp256k1.PrivateKey) (builder.Builder, walletsigner.Signer) {
	var (
		kc      = secp256k1fx.NewKeychain(keys...)
		addrs   = kc.Addresses()
		backend = newBackend(addrs, b.state, b.ctx.SharedMemory)
		context = newContext(b.ctx, b.cfg, b.state.GetTimestamp())
		builder = builder.New(addrs, context, backend)
		signer  = walletsigner.New(kc, backend)
	)

	return builder, signer
}
