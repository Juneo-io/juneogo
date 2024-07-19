// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/chain/p/builder"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"

	vmsigner "github.com/Juneo-io/juneogo/vms/platformvm/signer"
	walletsigner "github.com/Juneo-io/juneogo/wallet/chain/p/signer"
)

var _ Wallet = (*walletWithOptions)(nil)

func NewWalletWithOptions(
	wallet Wallet,
	options ...common.Option,
) Wallet {
	return &walletWithOptions{
		wallet:  wallet,
		options: options,
	}
}

type walletWithOptions struct {
	wallet  Wallet
	options []common.Option
}

func (w *walletWithOptions) Builder() builder.Builder {
	return builder.NewWithOptions(
		w.wallet.Builder(),
		w.options...,
	)
}

func (w *walletWithOptions) Signer() walletsigner.Signer {
	return w.wallet.Signer()
}

func (w *walletWithOptions) IssueBaseTx(
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueBaseTx(
		outputs,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddValidatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueAddValidatorTx(
		vdr,
		rewardsOwner,
		shares,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddSupernetValidatorTx(
	vdr *txs.SupernetValidator,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueAddSupernetValidatorTx(
		vdr,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueRemoveSupernetValidatorTx(
	nodeID ids.NodeID,
	supernetID ids.ID,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueRemoveSupernetValidatorTx(
		nodeID,
		supernetID,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddDelegatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueAddDelegatorTx(
		vdr,
		rewardsOwner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueCreateChainTx(
	supernetID ids.ID,
	genesis []byte,
	vmID ids.ID,
	fxIDs []ids.ID,
	chainName string,
	chainAssetID ids.ID,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueCreateChainTx(
		supernetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		chainAssetID,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueCreateSupernetTx(
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueCreateSupernetTx(
		owner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueTransferSupernetOwnershipTx(
	supernetID ids.ID,
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueTransferSupernetOwnershipTx(
		supernetID,
		owner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueImportTx(
	sourceChainID ids.ID,
	to *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueImportTx(
		sourceChainID,
		to,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueExportTx(
	chainID ids.ID,
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueExportTx(
		chainID,
		outputs,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueTransformSupernetTx(
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
) (*txs.Tx, error) {
	return w.wallet.IssueTransformSupernetTx(
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
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddPermissionlessValidatorTx(
	vdr *txs.SupernetValidator,
	signer vmsigner.Signer,
	assetID ids.ID,
	validationRewardsOwner *secp256k1fx.OutputOwners,
	delegationRewardsOwner *secp256k1fx.OutputOwners,
	shares uint32,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueAddPermissionlessValidatorTx(
		vdr,
		signer,
		assetID,
		validationRewardsOwner,
		delegationRewardsOwner,
		shares,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddPermissionlessDelegatorTx(
	vdr *txs.SupernetValidator,
	assetID ids.ID,
	rewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueAddPermissionlessDelegatorTx(
		vdr,
		assetID,
		rewardsOwner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueUnsignedTx(
	utx txs.UnsignedTx,
	options ...common.Option,
) (*txs.Tx, error) {
	return w.wallet.IssueUnsignedTx(
		utx,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueTx(
	tx *txs.Tx,
	options ...common.Option,
) error {
	return w.wallet.IssueTx(
		tx,
		common.UnionOptions(w.options, options)...,
	)
}
