// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
)

var (
	_ txs.Visitor = (*StandardTxExecutor)(nil)

	errEmptyNodeID              = errors.New("validator nodeID cannot be empty")
	errMaxStakeDurationTooLarge = errors.New("max stake duration must be less than or equal to the global max stake duration")
)

type StandardTxExecutor struct {
	// inputs, to be filled before visitor methods are called
	*Backend
	State state.Diff // state is expected to be modified
	Tx    *txs.Tx

	// outputs of visitor execution
	OnAccept       func() // may be nil
	Inputs         set.Set[ids.ID]
	AtomicRequests map[ids.ID]*atomic.Requests // may be nil
}

func (*StandardTxExecutor) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) RewardValidatorTx(*txs.RewardValidatorTx) error {
	return ErrWrongTxType
}

func (e *StandardTxExecutor) CreateChainTx(tx *txs.CreateChainTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	baseTxCreds, err := verifyPoASupernetAuthorization(e.Backend, e.State, e.Tx, tx.SupernetID, tx.SupernetAuth)
	if err != nil {
		return err
	}

	// Verify the flowcheck
	timestamp := e.State.GetTimestamp()
	createBlockchainTxFee := e.Config.GetCreateBlockchainTxFee(timestamp)
	if err := e.FlowChecker.VerifySpend(
		tx,
		e.State,
		tx.Ins,
		tx.Outs,
		baseTxCreds,
		map[ids.ID]uint64{
			e.Ctx.AVAXAssetID: createBlockchainTxFee,
		},
	); err != nil {
		return err
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)
	// Add the new chain to the database
	e.State.AddChain(e.Tx)

	// If this proposal is committed and this node is a member of the supernet
	// that validates the blockchain, create the blockchain
	e.OnAccept = func() {
		e.Config.CreateChain(txID, tx)
	}
	return nil
}

func (e *StandardTxExecutor) CreateSupernetTx(tx *txs.CreateSupernetTx) error {
	// Make sure this transaction is well formed.
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	// Verify the flowcheck
	timestamp := e.State.GetTimestamp()
	createSupernetTxFee := e.Config.GetCreateSupernetTxFee(timestamp)
	if err := e.FlowChecker.VerifySpend(
		tx,
		e.State,
		tx.Ins,
		tx.Outs,
		e.Tx.Creds,
		map[ids.ID]uint64{
			e.Ctx.AVAXAssetID: createSupernetTxFee,
		},
	); err != nil {
		return err
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)
	// Add the new supernet to the database
	e.State.AddSupernet(e.Tx)
	e.State.SetSupernetOwner(txID, tx.Owner)
	return nil
}

func (e *StandardTxExecutor) ImportTx(tx *txs.ImportTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	e.Inputs = set.NewSet[ids.ID](len(tx.ImportedInputs))
	utxoIDs := make([][]byte, len(tx.ImportedInputs))
	for i, in := range tx.ImportedInputs {
		utxoID := in.UTXOID.InputID()

		e.Inputs.Add(utxoID)
		utxoIDs[i] = utxoID[:]
	}

	// Skip verification of the shared memory inputs if the other primary
	// network chains are not guaranteed to be up-to-date.
	if e.Bootstrapped.Get() && !e.Config.PartialSyncPrimaryNetwork {
		if err := verify.SameSupernet(context.TODO(), e.Ctx, tx.SourceChain); err != nil {
			return err
		}

		allUTXOBytes, err := e.Ctx.SharedMemory.Get(tx.SourceChain, utxoIDs)
		if err != nil {
			return fmt.Errorf("failed to get shared memory: %w", err)
		}

		utxos := make([]*avax.UTXO, len(tx.Ins)+len(tx.ImportedInputs))
		for index, input := range tx.Ins {
			utxo, err := e.State.GetUTXO(input.InputID())
			if err != nil {
				return fmt.Errorf("failed to get UTXO %s: %w", &input.UTXOID, err) //nolint:gosec
			}
			utxos[index] = utxo
		}
		for i, utxoBytes := range allUTXOBytes {
			utxo := &avax.UTXO{}
			if _, err := txs.Codec.Unmarshal(utxoBytes, utxo); err != nil {
				return fmt.Errorf("failed to unmarshal UTXO: %w", err)
			}
			utxos[i+len(tx.Ins)] = utxo
		}

		ins := make([]*avax.TransferableInput, len(tx.Ins)+len(tx.ImportedInputs))
		copy(ins, tx.Ins)
		copy(ins[len(tx.Ins):], tx.ImportedInputs)

		if err := e.FlowChecker.VerifySpendUTXOs(
			tx,
			utxos,
			ins,
			tx.Outs,
			e.Tx.Creds,
			map[ids.ID]uint64{
				e.Ctx.AVAXAssetID: e.Config.TxFee,
			},
		); err != nil {
			return err
		}
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)

	// Note: We apply atomic requests even if we are not verifying atomic
	// requests to ensure the shared state will be correct if we later start
	// verifying the requests.
	e.AtomicRequests = map[ids.ID]*atomic.Requests{
		tx.SourceChain: {
			RemoveRequests: utxoIDs,
		},
	}
	return nil
}

func (e *StandardTxExecutor) ExportTx(tx *txs.ExportTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.ExportedOutputs))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.ExportedOutputs)

	if e.Bootstrapped.Get() {
		if err := verify.SameSupernet(context.TODO(), e.Ctx, tx.DestinationChain); err != nil {
			return err
		}
	}

	// Verify the flowcheck
	if err := e.FlowChecker.VerifySpend(
		tx,
		e.State,
		tx.Ins,
		outs,
		e.Tx.Creds,
		map[ids.ID]uint64{
			e.Ctx.AVAXAssetID: e.Config.TxFee,
		},
	); err != nil {
		return fmt.Errorf("failed verifySpend: %w", err)
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)

	// Note: We apply atomic requests even if we are not verifying atomic
	// requests to ensure the shared state will be correct if we later start
	// verifying the requests.
	elems := make([]*atomic.Element, len(tx.ExportedOutputs))
	for i, out := range tx.ExportedOutputs {
		utxo := &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(len(tx.Outs) + i),
			},
			Asset: avax.Asset{ID: out.AssetID()},
			Out:   out.Out,
		}

		utxoBytes, err := txs.Codec.Marshal(txs.Version, utxo)
		if err != nil {
			return fmt.Errorf("failed to marshal UTXO: %w", err)
		}
		utxoID := utxo.InputID()
		elem := &atomic.Element{
			Key:   utxoID[:],
			Value: utxoBytes,
		}
		if out, ok := utxo.Out.(avax.Addressable); ok {
			elem.Traits = out.Addresses()
		}

		elems[i] = elem
	}
	e.AtomicRequests = map[ids.ID]*atomic.Requests{
		tx.DestinationChain: {
			PutRequests: elems,
		},
	}
	return nil
}

func (e *StandardTxExecutor) AddValidatorTx(tx *txs.AddValidatorTx) error {
	if tx.Validator.NodeID == ids.EmptyNodeID {
		return errEmptyNodeID
	}

	if _, err := verifyAddValidatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	); err != nil {
		return err
	}

	txID := e.Tx.ID()
	newStaker, err := state.NewPendingStaker(txID, tx)
	if err != nil {
		return err
	}

	e.State.PutPendingValidator(newStaker)
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	if e.Config.PartialSyncPrimaryNetwork && tx.Validator.NodeID == e.Ctx.NodeID {
		e.Ctx.Log.Warn("verified transaction that would cause this node to become unhealthy",
			zap.String("reason", "primary network is not being fully synced"),
			zap.Stringer("txID", txID),
			zap.String("txType", "addValidator"),
			zap.Stringer("nodeID", tx.Validator.NodeID),
		)
	}
	return nil
}

func (e *StandardTxExecutor) AddSupernetValidatorTx(tx *txs.AddSupernetValidatorTx) error {
	if err := verifyAddSupernetValidatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	); err != nil {
		return err
	}

	txID := e.Tx.ID()
	newStaker, err := state.NewPendingStaker(txID, tx)
	if err != nil {
		return err
	}

	e.State.PutPendingValidator(newStaker)
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	return nil
}

func (e *StandardTxExecutor) AddDelegatorTx(tx *txs.AddDelegatorTx) error {
	if _, err := verifyAddDelegatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	); err != nil {
		return err
	}

	txID := e.Tx.ID()
	newStaker, err := state.NewPendingStaker(txID, tx)
	if err != nil {
		return err
	}

	e.State.PutPendingDelegator(newStaker)
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	return nil
}

// Verifies a [*txs.RemoveSupernetValidatorTx] and, if it passes, executes it on
// [e.State]. For verification rules, see [verifyRemoveSupernetValidatorTx]. This
// transaction will result in [tx.NodeID] being removed as a validator of
// [tx.SupernetID].
// Note: [tx.NodeID] may be either a current or pending validator.
func (e *StandardTxExecutor) RemoveSupernetValidatorTx(tx *txs.RemoveSupernetValidatorTx) error {
	staker, isCurrentValidator, err := verifyRemoveSupernetValidatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	)
	if err != nil {
		return err
	}

	if isCurrentValidator {
		e.State.DeleteCurrentValidator(staker)
	} else {
		e.State.DeletePendingValidator(staker)
	}

	// Invariant: There are no permissioned supernet delegators to remove.

	txID := e.Tx.ID()
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	return nil
}

func (e *StandardTxExecutor) TransformSupernetTx(tx *txs.TransformSupernetTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	// Note: math.MaxInt32 * time.Second < math.MaxInt64 - so this can never
	// overflow.
	if time.Duration(tx.MaxStakeDuration)*time.Second > e.Backend.Config.MaxStakeDuration {
		return errMaxStakeDurationTooLarge
	}

	baseTxCreds, err := verifyPoASupernetAuthorization(e.Backend, e.State, e.Tx, tx.Supernet, tx.SupernetAuth)
	if err != nil {
		return err
	}

	totalRewardAmount := tx.InitialRewardPoolSupply
	if err := e.Backend.FlowChecker.VerifySpend(
		tx,
		e.State,
		tx.Ins,
		tx.Outs,
		baseTxCreds,
		// Invariant: [tx.AssetID != e.Ctx.AVAXAssetID]. This prevents the first
		//            entry in this map literal from being overwritten by the
		//            second entry.
		map[ids.ID]uint64{
			e.Ctx.AVAXAssetID: e.Config.TransformSupernetTxFee,
			tx.AssetID:        totalRewardAmount,
		},
	); err != nil {
		return err
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)
	// Transform the new supernet in the database
	e.State.AddSupernetTransformation(e.Tx)
	e.State.SetCurrentSupply(tx.Supernet, uint64(0))
	e.State.SetRewardPoolSupply(tx.Supernet, totalRewardAmount)
	return nil
}

func (e *StandardTxExecutor) AddPermissionlessValidatorTx(tx *txs.AddPermissionlessValidatorTx) error {
	if err := verifyAddPermissionlessValidatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	); err != nil {
		return err
	}

	txID := e.Tx.ID()
	newStaker, err := state.NewPendingStaker(txID, tx)
	if err != nil {
		return err
	}

	e.State.PutPendingValidator(newStaker)
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	if e.Config.PartialSyncPrimaryNetwork &&
		tx.Supernet == constants.PrimaryNetworkID &&
		tx.Validator.NodeID == e.Ctx.NodeID {
		e.Ctx.Log.Warn("verified transaction that would cause this node to become unhealthy",
			zap.String("reason", "primary network is not being fully synced"),
			zap.Stringer("txID", txID),
			zap.String("txType", "addPermissionlessValidator"),
			zap.Stringer("nodeID", tx.Validator.NodeID),
		)
	}

	return nil
}

func (e *StandardTxExecutor) AddPermissionlessDelegatorTx(tx *txs.AddPermissionlessDelegatorTx) error {
	if err := verifyAddPermissionlessDelegatorTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	); err != nil {
		return err
	}

	txID := e.Tx.ID()
	newStaker, err := state.NewPendingStaker(txID, tx)
	if err != nil {
		return err
	}

	e.State.PutPendingDelegator(newStaker)
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	return nil
}

// Verifies a [*txs.TransferSupernetOwnershipTx] and, if it passes, executes it on
// [e.State]. For verification rules, see [verifyTransferSupernetOwnershipTx].
// This transaction will result in the ownership of [tx.Supernet] being transferred
// to [tx.Owner].
func (e *StandardTxExecutor) TransferSupernetOwnershipTx(tx *txs.TransferSupernetOwnershipTx) error {
	err := verifyTransferSupernetOwnershipTx(
		e.Backend,
		e.State,
		e.Tx,
		tx,
	)
	if err != nil {
		return err
	}

	e.State.SetSupernetOwner(tx.Supernet, tx.Owner)

	txID := e.Tx.ID()
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)

	return nil
}
