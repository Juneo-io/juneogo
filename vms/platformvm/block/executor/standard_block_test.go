// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/block"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs/executor"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestApricotStandardBlockTimeVerification(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	env := newEnvironment(t, ctrl, apricotPhase5)

	// setup and store parent block
	// it's a standard block for simplicity
	parentHeight := uint64(2022)

	apricotParentBlk, err := block.NewApricotStandardBlock(
		ids.Empty, // does not matter
		parentHeight,
		nil, // txs do not matter in this test
	)
	require.NoError(err)
	parentID := apricotParentBlk.ID()

	// store parent block, with relevant quantities
	onParentAccept := state.NewMockDiff(ctrl)
	env.blkManager.(*manager).blkIDToState[parentID] = &blockState{
		statelessBlock: apricotParentBlk,
		onAcceptState:  onParentAccept,
	}
	env.blkManager.(*manager).lastAccepted = parentID

	chainTime := env.clk.Time().Truncate(time.Second)
	env.mockedState.EXPECT().GetLastAccepted().Return(parentID).AnyTimes()
	env.mockedState.EXPECT().GetTimestamp().Return(chainTime).AnyTimes()
	onParentAccept.EXPECT().GetTimestamp().Return(chainTime).AnyTimes()
	onParentAccept.EXPECT().GetFeePoolValue().Return(uint64(0)).AnyTimes()

	// wrong height
	apricotChildBlk, err := block.NewApricotStandardBlock(
		apricotParentBlk.ID(),
		apricotParentBlk.Height(),
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	blk := env.blkManager.NewBlock(apricotChildBlk)
	err = blk.Verify(context.Background())
	require.ErrorIs(err, errIncorrectBlockHeight)

	// valid height
	apricotChildBlk, err = block.NewApricotStandardBlock(
		apricotParentBlk.ID(),
		apricotParentBlk.Height()+1,
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	blk = env.blkManager.NewBlock(apricotChildBlk)
	require.NoError(blk.Verify(context.Background()))
}

func TestBanffStandardBlockTimeVerification(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	env := newEnvironment(t, ctrl, banff)
	now := env.clk.Time()
	env.clk.Set(now)

	// setup and store parent block
	// it's a standard block for simplicity
	parentTime := now
	parentHeight := uint64(2022)

	banffParentBlk, err := block.NewBanffStandardBlock(
		parentTime,
		ids.Empty, // does not matter
		parentHeight,
		nil, // txs do not matter in this test
	)
	require.NoError(err)
	parentID := banffParentBlk.ID()

	// store parent block, with relevant quantities
	onParentAccept := state.NewMockDiff(ctrl)
	chainTime := env.clk.Time().Truncate(time.Second)
	env.blkManager.(*manager).blkIDToState[parentID] = &blockState{
		statelessBlock: banffParentBlk,
		onAcceptState:  onParentAccept,
		timestamp:      chainTime,
	}
	env.blkManager.(*manager).lastAccepted = parentID
	env.mockedState.EXPECT().GetLastAccepted().Return(parentID).AnyTimes()
	env.mockedState.EXPECT().GetTimestamp().Return(chainTime).AnyTimes()

	nextStakerTime := chainTime.Add(executor.SyncBound).Add(-1 * time.Second)

	// store just once current staker to mark next staker time.
	currentStakerIt := state.NewMockStakerIterator(ctrl)
	currentStakerIt.EXPECT().Next().Return(true).AnyTimes()
	currentStakerIt.EXPECT().Value().Return(
		&state.Staker{
			NextTime: nextStakerTime,
			Priority: txs.PrimaryNetworkValidatorCurrentPriority,
		},
	).AnyTimes()
	currentStakerIt.EXPECT().Release().Return().AnyTimes()
	onParentAccept.EXPECT().GetCurrentStakerIterator().Return(currentStakerIt, nil).AnyTimes()

	// no pending stakers
	pendingIt := state.NewMockStakerIterator(ctrl)
	pendingIt.EXPECT().Next().Return(false).AnyTimes()
	pendingIt.EXPECT().Release().Return().AnyTimes()
	onParentAccept.EXPECT().GetPendingStakerIterator().Return(pendingIt, nil).AnyTimes()

	onParentAccept.EXPECT().GetTimestamp().Return(chainTime).AnyTimes()
	onParentAccept.EXPECT().GetFeePoolValue().Return(uint64(0)).AnyTimes()

	txID := ids.GenerateTestID()
	utxo := &avax.UTXO{
		UTXOID: avax.UTXOID{
			TxID: txID,
		},
		Asset: avax.Asset{
			ID: juneAssetID,
		},
		Out: &secp256k1fx.TransferOutput{
			Amt: env.config.CreateSupernetTxFee,
		},
	}
	utxoID := utxo.InputID()
	onParentAccept.EXPECT().GetUTXO(utxoID).Return(utxo, nil).AnyTimes()

	// Create the tx
	utx := &txs.CreateSupernetTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    env.ctx.NetworkID,
			BlockchainID: env.ctx.ChainID,
			Ins: []*avax.TransferableInput{{
				UTXOID: utxo.UTXOID,
				Asset:  utxo.Asset,
				In: &secp256k1fx.TransferInput{
					Amt: env.config.CreateSupernetTxFee,
				},
			}},
		}},
		Owner: &secp256k1fx.OutputOwners{},
	}
	tx := &txs.Tx{Unsigned: utx}
	require.NoError(tx.Sign(txs.Codec, [][]*secp256k1.PrivateKey{{}}))

	{
		// wrong version
		banffChildBlk, err := block.NewApricotStandardBlock(
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, errApricotBlockIssuedAfterFork)
	}

	{
		// wrong height
		childTimestamp := parentTime.Add(time.Second)
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height(),
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, errIncorrectBlockHeight)
	}

	{
		// wrong timestamp, earlier than parent
		childTimestamp := parentTime.Add(-1 * time.Second)
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, errChildBlockEarlierThanParent)
	}

	{
		// wrong timestamp, violated synchrony bound
		initClkTime := env.clk.Time()
		env.clk.Set(parentTime.Add(-executor.SyncBound))
		banffChildBlk, err := block.NewBanffStandardBlock(
			parentTime.Add(time.Second),
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, executor.ErrChildBlockBeyondSyncBound)
		env.clk.Set(initClkTime)
	}

	{
		// wrong timestamp, skipped staker set change event
		childTimestamp := nextStakerTime.Add(time.Second)
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, executor.ErrChildBlockAfterStakerChangeTime)
	}

	{
		// no state changes
		childTimestamp := parentTime
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			nil,
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		err = block.Verify(context.Background())
		require.ErrorIs(err, errBanffStandardBlockWithoutChanges)
	}

	{
		// valid block, same timestamp as parent block
		childTimestamp := parentTime
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		require.NoError(block.Verify(context.Background()))
	}

	{
		// valid
		childTimestamp := nextStakerTime
		banffChildBlk, err := block.NewBanffStandardBlock(
			childTimestamp,
			banffParentBlk.ID(),
			banffParentBlk.Height()+1,
			[]*txs.Tx{tx},
		)
		require.NoError(err)
		block := env.blkManager.NewBlock(banffChildBlk)
		require.NoError(block.Verify(context.Background()))
	}
}

func TestBanffStandardBlockUpdatePrimaryNetworkStakers(t *testing.T) {
	require := require.New(t)

	env := newEnvironment(t, nil, banff)

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultGenesisTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	rewardAddress := ids.GenerateTestShortID()
	addPendingValidatorTx, err := addPendingValidator(
		env,
		pendingValidatorStartTime,
		pendingValidatorEndTime,
		nodeID,
		rewardAddress,
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
	)
	require.NoError(err)

	// build standard block moving ahead chain time
	preferredID := env.state.GetLastAccepted()
	parentBlk, err := env.state.GetStatelessBlock(preferredID)
	require.NoError(err)
	statelessStandardBlock, err := block.NewBanffStandardBlock(
		pendingValidatorStartTime,
		parentBlk.ID(),
		parentBlk.Height()+1,
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	block := env.blkManager.NewBlock(statelessStandardBlock)

	// update staker set
	require.NoError(block.Verify(context.Background()))

	// tests
	blkStateMap := env.blkManager.(*manager).blkIDToState
	updatedState := blkStateMap[block.ID()].onAcceptState
	currentValidator, err := updatedState.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)
	require.Equal(addPendingValidatorTx.ID(), currentValidator.TxID)
	require.Equal(uint64(1640), currentValidator.PotentialReward) // See rewards tests to explain why 1370

	_, err = updatedState.GetPendingValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// Test VM validators
	require.NoError(block.Accept(context.Background()))
	_, ok := env.config.Validators.GetValidator(constants.PrimaryNetworkID, nodeID)
	require.True(ok)
}

// Ensure semantic verification updates the current and pending staker sets correctly.
// Namely, it should add pending stakers whose start time is at or before the timestamp.
// It will not remove primary network stakers; that happens in rewardTxs.
func TestBanffStandardBlockUpdateStakers(t *testing.T) {
	// Chronological order (not in scale):
	// Staker1:    |----------------------------------------------------------|
	// Staker2:        |------------------------|
	// Staker3:            |------------------------|
	// Staker3sub:             |----------------|
	// Staker4:            |------------------------|
	// Staker5:                                 |--------------------|

	// In this test multiple stakers may join and leave the staker set at the same time.
	// The order in which they do it is asserted; the order may depend on the staker.TxID,
	// which in turns depend on every feature of the transaction creating the staker.
	// So in this test we avoid ids.GenerateTestNodeID, in favour of ids.BuildTestNodeID
	// so that TxID does not depend on the order we run tests.
	staker1 := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf1}),
		rewardAddress: ids.ShortID{0xf1},
		startTime:     defaultGenesisTime.Add(1 * time.Minute),
		endTime:       defaultGenesisTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute),
	}
	staker2 := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf2}),
		rewardAddress: ids.ShortID{0xf2},
		startTime:     staker1.startTime.Add(1 * time.Minute),
		endTime:       staker1.startTime.Add(1 * time.Minute).Add(defaultMinStakingDuration),
	}
	staker3 := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf3}),
		rewardAddress: ids.ShortID{0xf3},
		startTime:     staker2.startTime.Add(1 * time.Minute),
		endTime:       staker2.endTime.Add(1 * time.Minute),
	}
	staker3Sub := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf3}),
		rewardAddress: ids.ShortID{0xff},
		startTime:     staker3.startTime.Add(1 * time.Minute),
		endTime:       staker3.endTime.Add(-1 * time.Minute),
	}
	staker4 := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf4}),
		rewardAddress: ids.ShortID{0xf4},
		startTime:     staker3.startTime,
		endTime:       staker3.endTime,
	}
	staker5 := staker{
		nodeID:        ids.BuildTestNodeID([]byte{0xf5}),
		rewardAddress: ids.ShortID{0xf5},
		startTime:     staker2.endTime,
		endTime:       staker2.endTime.Add(defaultMinStakingDuration),
	}

	tests := []test{
		{
			description:   "advance time to staker 1 start with supernet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			supernetStakers: []staker{staker1},
			advanceTimeTo: []time.Time{staker1.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: pending,
				staker3.nodeID: pending,
				staker4.nodeID: pending,
				staker5.nodeID: pending,
			},
			expectedSupernetStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: pending,
				staker3.nodeID: pending,
				staker4.nodeID: pending,
				staker5.nodeID: pending,
			},
		},
		{
			description:   "advance time to the staker2 start",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: current,
				staker3.nodeID: pending,
				staker4.nodeID: pending,
				staker5.nodeID: pending,
			},
		},
		{
			description:   "staker3 should validate only primary network",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			supernetStakers: []staker{staker1, staker2, staker3Sub, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: current,
				staker3.nodeID: current,
				staker4.nodeID: current,
				staker5.nodeID: pending,
			},
			expectedSupernetStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID:    current,
				staker2.nodeID:    current,
				staker3Sub.nodeID: pending,
				staker4.nodeID:    current,
				staker5.nodeID:    pending,
			},
		},
		{
			description:   "advance time to staker3 start with supernet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			supernetStakers: []staker{staker1, staker2, staker3Sub, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime, staker3Sub.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: current,
				staker3.nodeID: current,
				staker4.nodeID: current,
				staker5.nodeID: pending,
			},
			expectedSupernetStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
				staker2.nodeID: current,
				staker3.nodeID: current,
				staker4.nodeID: current,
				staker5.nodeID: pending,
			},
		},
		{
			description:   "advance time to staker5 start",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime, staker5.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,

				// Staker2's end time matches staker5's start time, so typically
				// the block builder would produce a ProposalBlock to remove
				// staker2 when advancing the time. However, it is valid to only
				// advance the time with a StandardBlock and not remove staker2,
				// which is what this test does.
				staker2.nodeID: current,
				staker3.nodeID: current,
				staker4.nodeID: current,
				staker5.nodeID: current,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			require := require.New(t)
			env := newEnvironment(t, nil, banff)

			supernetID := testSupernet1.ID()
			env.config.TrackedSupernets.Add(supernetID)

			for _, staker := range test.stakers {
				_, err := addPendingValidator(
					env,
					staker.startTime,
					staker.endTime,
					staker.nodeID,
					staker.rewardAddress,
					[]*secp256k1.PrivateKey{preFundedKeys[0]},
				)
				require.NoError(err)
			}

			for _, staker := range test.supernetStakers {
				tx, err := env.txBuilder.NewAddSupernetValidatorTx(
					&txs.SupernetValidator{
						Validator: txs.Validator{
							NodeID: staker.nodeID,
							Start:  uint64(staker.startTime.Unix()),
							End:    uint64(staker.endTime.Unix()),
							Wght:   10,
						},
						Supernet: supernetID,
					},
					[]*secp256k1.PrivateKey{preFundedKeys[0], preFundedKeys[1]},
				)
				require.NoError(err)

				staker, err := state.NewPendingStaker(
					tx.ID(),
					tx.Unsigned.(*txs.AddSupernetValidatorTx),
				)
				require.NoError(err)

				env.state.PutPendingValidator(staker)
				env.state.AddTx(tx, status.Committed)
			}
			env.state.SetHeight( /*dummyHeight*/ 1)
			require.NoError(env.state.Commit())

			for _, newTime := range test.advanceTimeTo {
				env.clk.Set(newTime)

				// build standard block moving ahead chain time
				preferredID := env.state.GetLastAccepted()
				parentBlk, err := env.state.GetStatelessBlock(preferredID)
				require.NoError(err)
				statelessStandardBlock, err := block.NewBanffStandardBlock(
					newTime,
					parentBlk.ID(),
					parentBlk.Height()+1,
					nil, // txs nulled to simplify test
				)
				block := env.blkManager.NewBlock(statelessStandardBlock)

				require.NoError(err)

				// update staker set
				require.NoError(block.Verify(context.Background()))
				require.NoError(block.Accept(context.Background()))
			}

			for stakerNodeID, status := range test.expectedStakers {
				switch status {
				case pending:
					_, err := env.state.GetPendingValidator(constants.PrimaryNetworkID, stakerNodeID)
					require.NoError(err)
					_, ok := env.config.Validators.GetValidator(constants.PrimaryNetworkID, stakerNodeID)
					require.False(ok)
				case current:
					_, err := env.state.GetCurrentValidator(constants.PrimaryNetworkID, stakerNodeID)
					require.NoError(err)
					_, ok := env.config.Validators.GetValidator(constants.PrimaryNetworkID, stakerNodeID)
					require.True(ok)
				}
			}

			for stakerNodeID, status := range test.expectedSupernetStakers {
				switch status {
				case pending:
					_, ok := env.config.Validators.GetValidator(supernetID, stakerNodeID)
					require.False(ok)
				case current:
					_, ok := env.config.Validators.GetValidator(supernetID, stakerNodeID)
					require.True(ok)
				}
			}
		})
	}
}

// Regression test for https://github.com/ava-labs/avalanchego/pull/584
// that ensures it fixes a bug where supernet validators are not removed
// when timestamp is advanced and there is a pending staker whose start time
// is after the new timestamp
func TestBanffStandardBlockRemoveSupernetValidator(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, nil, banff)

	supernetID := testSupernet1.ID()
	env.config.TrackedSupernets.Add(supernetID)

	// Add a supernet validator to the staker set
	supernetValidatorNodeID := genesisNodeIDs[0]
	supernetVdr1StartTime := defaultValidateStartTime
	supernetVdr1EndTime := defaultValidateStartTime.Add(defaultMinStakingDuration)
	tx, err := env.txBuilder.NewAddSupernetValidatorTx(
		&txs.SupernetValidator{
			Validator: txs.Validator{
				NodeID: supernetValidatorNodeID,
				Start:  uint64(supernetVdr1StartTime.Unix()),
				End:    uint64(supernetVdr1EndTime.Unix()),
				Wght:   1,
			},
			Supernet: supernetID,
		},
		[]*secp256k1.PrivateKey{preFundedKeys[0], preFundedKeys[1]},
	)
	require.NoError(err)

	addSupernetValTx := tx.Unsigned.(*txs.AddSupernetValidatorTx)
	staker, err := state.NewCurrentStaker(
		tx.ID(),
		addSupernetValTx,
		addSupernetValTx.StartTime(),
		0,
	)
	require.NoError(err)

	env.state.PutCurrentValidator(staker)
	env.state.AddTx(tx, status.Committed)
	require.NoError(env.state.Commit())

	// The above validator is now part of the staking set

	// Queue a staker that joins the staker set after the above validator leaves
	supernetVdr2NodeID := genesisNodeIDs[1]
	tx, err = env.txBuilder.NewAddSupernetValidatorTx(
		&txs.SupernetValidator{
			Validator: txs.Validator{
				NodeID: supernetVdr2NodeID,
				Start:  uint64(supernetVdr1EndTime.Add(time.Second).Unix()),
				End:    uint64(supernetVdr1EndTime.Add(time.Second).Add(defaultMinStakingDuration).Unix()),
				Wght:   1,
			},
			Supernet: supernetID,
		},
		[]*secp256k1.PrivateKey{preFundedKeys[0], preFundedKeys[1]},
	)
	require.NoError(err)

	staker, err = state.NewPendingStaker(
		tx.ID(),
		tx.Unsigned.(*txs.AddSupernetValidatorTx),
	)
	require.NoError(err)

	env.state.PutPendingValidator(staker)
	env.state.AddTx(tx, status.Committed)
	require.NoError(env.state.Commit())

	// The above validator is now in the pending staker set

	// Advance time to the first staker's end time.
	env.clk.Set(supernetVdr1EndTime)
	// build standard block moving ahead chain time
	preferredID := env.state.GetLastAccepted()
	parentBlk, err := env.state.GetStatelessBlock(preferredID)
	require.NoError(err)
	statelessStandardBlock, err := block.NewBanffStandardBlock(
		supernetVdr1EndTime,
		parentBlk.ID(),
		parentBlk.Height()+1,
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	block := env.blkManager.NewBlock(statelessStandardBlock)

	// update staker set
	require.NoError(block.Verify(context.Background()))

	blkStateMap := env.blkManager.(*manager).blkIDToState
	updatedState := blkStateMap[block.ID()].onAcceptState
	_, err = updatedState.GetCurrentValidator(supernetID, supernetValidatorNodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// Check VM Validators are removed successfully
	require.NoError(block.Accept(context.Background()))
	_, ok := env.config.Validators.GetValidator(supernetID, supernetVdr2NodeID)
	require.False(ok)
	_, ok = env.config.Validators.GetValidator(supernetID, supernetValidatorNodeID)
	require.False(ok)
}

func TestBanffStandardBlockTrackedSupernet(t *testing.T) {
	for _, tracked := range []bool{true, false} {
		t.Run(fmt.Sprintf("tracked %t", tracked), func(t *testing.T) {
			require := require.New(t)
			env := newEnvironment(t, nil, banff)

			supernetID := testSupernet1.ID()
			if tracked {
				env.config.TrackedSupernets.Add(supernetID)
			}

			// Add a supernet validator to the staker set
			supernetValidatorNodeID := genesisNodeIDs[0]
			supernetVdr1StartTime := defaultGenesisTime.Add(1 * time.Minute)
			supernetVdr1EndTime := defaultGenesisTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute)
			tx, err := env.txBuilder.NewAddSupernetValidatorTx(
				&txs.SupernetValidator{
					Validator: txs.Validator{
						NodeID: supernetValidatorNodeID,
						Start:  uint64(supernetVdr1StartTime.Unix()),
						End:    uint64(supernetVdr1EndTime.Unix()),
						Wght:   1,
					},
					Supernet: supernetID,
				},
				[]*secp256k1.PrivateKey{preFundedKeys[0], preFundedKeys[1]},
			)
			require.NoError(err)

			staker, err := state.NewPendingStaker(
				tx.ID(),
				tx.Unsigned.(*txs.AddSupernetValidatorTx),
			)
			require.NoError(err)

			env.state.PutPendingValidator(staker)
			env.state.AddTx(tx, status.Committed)
			require.NoError(env.state.Commit())

			// Advance time to the staker's start time.
			env.clk.Set(supernetVdr1StartTime)

			// build standard block moving ahead chain time
			preferredID := env.state.GetLastAccepted()
			parentBlk, err := env.state.GetStatelessBlock(preferredID)
			require.NoError(err)
			statelessStandardBlock, err := block.NewBanffStandardBlock(
				supernetVdr1StartTime,
				parentBlk.ID(),
				parentBlk.Height()+1,
				nil, // txs nulled to simplify test
			)
			require.NoError(err)
			block := env.blkManager.NewBlock(statelessStandardBlock)

			// update staker set
			require.NoError(block.Verify(context.Background()))
			require.NoError(block.Accept(context.Background()))
			_, ok := env.config.Validators.GetValidator(supernetID, supernetValidatorNodeID)
			require.True(ok)
		})
	}
}

func TestBanffStandardBlockDelegatorStakerWeight(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, nil, banff)

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultGenesisTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMaxStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	rewardAddress := ids.GenerateTestShortID()
	_, err := addPendingValidator(
		env,
		pendingValidatorStartTime,
		pendingValidatorEndTime,
		nodeID,
		rewardAddress,
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
	)
	require.NoError(err)

	// build standard block moving ahead chain time
	preferredID := env.state.GetLastAccepted()
	parentBlk, err := env.state.GetStatelessBlock(preferredID)
	require.NoError(err)
	statelessStandardBlock, err := block.NewBanffStandardBlock(
		pendingValidatorStartTime,
		parentBlk.ID(),
		parentBlk.Height()+1,
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	blk := env.blkManager.NewBlock(statelessStandardBlock)
	require.NoError(blk.Verify(context.Background()))
	require.NoError(blk.Accept(context.Background()))
	require.NoError(env.state.Commit())

	// Test validator weight before delegation
	vdrWeight := env.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
	require.Equal(env.config.MinValidatorStake, vdrWeight)

	// Add delegator
	pendingDelegatorStartTime := pendingValidatorStartTime.Add(1 * time.Second)
	pendingDelegatorEndTime := pendingDelegatorStartTime.Add(1 * time.Second)

	addDelegatorTx, err := env.txBuilder.NewAddDelegatorTx(
		&txs.Validator{
			NodeID: nodeID,
			Start:  uint64(pendingDelegatorStartTime.Unix()),
			End:    uint64(pendingDelegatorEndTime.Unix()),
			Wght:   env.config.MinDelegatorStake,
		},
		&secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     []ids.ShortID{preFundedKeys[0].PublicKey().Address()},
		},
		[]*secp256k1.PrivateKey{
			preFundedKeys[0],
			preFundedKeys[1],
			preFundedKeys[4],
		},
	)
	require.NoError(err)

	staker, err := state.NewPendingStaker(
		addDelegatorTx.ID(),
		addDelegatorTx.Unsigned.(*txs.AddDelegatorTx),
	)
	require.NoError(err)

	env.state.PutPendingDelegator(staker)
	env.state.AddTx(addDelegatorTx, status.Committed)
	env.state.SetHeight( /*dummyHeight*/ uint64(1))
	require.NoError(env.state.Commit())

	// Advance Time
	preferredID = env.state.GetLastAccepted()
	parentBlk, err = env.state.GetStatelessBlock(preferredID)
	require.NoError(err)
	statelessStandardBlock, err = block.NewBanffStandardBlock(
		pendingDelegatorStartTime,
		parentBlk.ID(),
		parentBlk.Height()+1,
		nil, // txs nulled to simplify test
	)
	require.NoError(err)
	blk = env.blkManager.NewBlock(statelessStandardBlock)
	require.NoError(blk.Verify(context.Background()))
	require.NoError(blk.Accept(context.Background()))
	require.NoError(env.state.Commit())

	// Test validator weight after delegation
	vdrWeight = env.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
	require.Equal(env.config.MinDelegatorStake+env.config.MinValidatorStake, vdrWeight)
}
