// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/snowtest"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func newAdvanceTimeTx(t testing.TB, timestamp time.Time) (*txs.Tx, error) {
	utx := &txs.AdvanceTimeTx{Time: uint64(timestamp.Unix())}
	tx, err := txs.NewSigned(utx, txs.Codec, nil)
	if err != nil {
		return nil, err
	}
	return tx, tx.SyntacticVerify(snowtest.Context(t, snowtest.PChainID))
}

// Ensure semantic verification updates the current and pending staker set
// for the primary network
func TestAdvanceTimeTxUpdatePrimaryNetworkStakers(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()
	dummyHeight := uint64(1)

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultValidateStartTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	addPendingValidatorTx, err := addPendingValidator(
		env,
		pendingValidatorStartTime,
		pendingValidatorEndTime,
		nodeID,
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
	)
	require.NoError(err)

	tx, err := newAdvanceTimeTx(t, pendingValidatorStartTime)
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	validatorStaker, err := executor.OnCommitState.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)
	require.Equal(addPendingValidatorTx.ID(), validatorStaker.TxID)
	require.Equal(uint64(1640), validatorStaker.PotentialReward) // See rewards tests to explain why 1640

	_, err = executor.OnCommitState.GetPendingValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	_, err = executor.OnAbortState.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	validatorStaker, err = executor.OnAbortState.GetPendingValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)
	require.Equal(addPendingValidatorTx.ID(), validatorStaker.TxID)

	// Test VM validators
	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())
	_, ok := env.config.Validators.GetValidator(constants.PrimaryNetworkID, nodeID)
	require.True(ok)
}

// Ensure semantic verification fails when proposed timestamp is at or before current timestamp
func TestAdvanceTimeTxTimestampTooEarly(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)

	tx, err := newAdvanceTimeTx(t, env.state.GetTimestamp())
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	err = tx.Unsigned.Visit(&executor)
	require.ErrorIs(err, ErrChildBlockNotAfterParent)
}

// Ensure semantic verification fails when proposed timestamp is after next validator set change time
func TestAdvanceTimeTxTimestampTooLate(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultValidateStartTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	_, err := addPendingValidator(env, pendingValidatorStartTime, pendingValidatorEndTime, nodeID, []*secp256k1.PrivateKey{preFundedKeys[0]})
	require.NoError(err)

	{
		tx, err := newAdvanceTimeTx(t, pendingValidatorStartTime.Add(1*time.Second))
		require.NoError(err)

		onCommitState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		onAbortState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := ProposalTxExecutor{
			OnCommitState: onCommitState,
			OnAbortState:  onAbortState,
			Backend:       &env.backend,
			Tx:            tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.ErrorIs(err, ErrChildBlockAfterStakerChangeTime)
	}

	// Case: Timestamp is after next validator end time
	env = newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()

	// fast forward clock to 10 seconds before genesis validators stop validating
	env.clk.Set(defaultValidateEndTime.Add(-10 * time.Second))

	{
		// Proposes advancing timestamp to 1 second after genesis validators stop validating
		tx, err := newAdvanceTimeTx(t, defaultValidateEndTime.Add(1*time.Second))
		require.NoError(err)

		onCommitState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		onAbortState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := ProposalTxExecutor{
			OnCommitState: onCommitState,
			OnAbortState:  onAbortState,
			Backend:       &env.backend,
			Tx:            tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.ErrorIs(err, ErrChildBlockAfterStakerChangeTime)
	}
}

// Ensure semantic verification updates the current and pending staker sets correctly.
// Namely, it should add pending stakers whose start time is at or before the timestamp.
// It will not remove primary network stakers; that happens in rewardTxs.
func TestAdvanceTimeTxUpdateStakers(t *testing.T) {
	type stakerStatus uint
	const (
		pending stakerStatus = iota
		current
	)

	type staker struct {
		nodeID             ids.NodeID
		startTime, endTime time.Time
	}
	type test struct {
		description           string
		stakers               []staker
		supernetStakers         []staker
		advanceTimeTo         []time.Time
		expectedStakers       map[ids.NodeID]stakerStatus
		expectedSupernetStakers map[ids.NodeID]stakerStatus
	}

	// Chronological order (not in scale):
	// Staker1:    |----------------------------------------------------------|
	// Staker2:        |------------------------|
	// Staker3:            |------------------------|
	// Staker3sub:             |----------------|
	// Staker4:            |------------------------|
	// Staker5:                                 |--------------------|
	staker1 := staker{
		nodeID:    ids.GenerateTestNodeID(),
		startTime: defaultValidateStartTime.Add(1 * time.Minute),
		endTime:   defaultValidateStartTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute),
	}
	staker2 := staker{
		nodeID:    ids.GenerateTestNodeID(),
		startTime: staker1.startTime.Add(1 * time.Minute),
		endTime:   staker1.startTime.Add(1 * time.Minute).Add(defaultMinStakingDuration),
	}
	staker3 := staker{
		nodeID:    ids.GenerateTestNodeID(),
		startTime: staker2.startTime.Add(1 * time.Minute),
		endTime:   staker2.endTime.Add(1 * time.Minute),
	}
	staker3Sub := staker{
		nodeID:    staker3.nodeID,
		startTime: staker3.startTime.Add(1 * time.Minute),
		endTime:   staker3.endTime.Add(-1 * time.Minute),
	}
	staker4 := staker{
		nodeID:    ids.GenerateTestNodeID(),
		startTime: staker3.startTime,
		endTime:   staker3.endTime,
	}
	staker5 := staker{
		nodeID:    ids.GenerateTestNodeID(),
		startTime: staker2.endTime,
		endTime:   staker2.endTime.Add(defaultMinStakingDuration),
	}

	tests := []test{
		{
			description:   "advance time to before staker1 start with supernet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			supernetStakers: []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime.Add(-1 * time.Second)},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: pending,
				staker2.nodeID: pending,
				staker3.nodeID: pending,
				staker4.nodeID: pending,
				staker5.nodeID: pending,
			},
			expectedSupernetStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: pending,
				staker2.nodeID: pending,
				staker3.nodeID: pending,
				staker4.nodeID: pending,
				staker5.nodeID: pending,
			},
		},
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
			description:   "advance time to staker5 end",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime, staker5.startTime},
			expectedStakers: map[ids.NodeID]stakerStatus{
				staker1.nodeID: current,
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
			env := newEnvironment(t, apricotPhase5)
			env.ctx.Lock.Lock()
			defer env.ctx.Lock.Unlock()

			dummyHeight := uint64(1)

			supernetID := testSupernet1.ID()
			env.config.TrackedSupernets.Add(supernetID)

			for _, staker := range test.stakers {
				_, err := addPendingValidator(
					env,
					staker.startTime,
					staker.endTime,
					staker.nodeID,
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
			env.state.SetHeight(dummyHeight)
			require.NoError(env.state.Commit())

			for _, newTime := range test.advanceTimeTo {
				env.clk.Set(newTime)
				tx, err := newAdvanceTimeTx(t, newTime)
				require.NoError(err)

				onCommitState, err := state.NewDiff(lastAcceptedID, env)
				require.NoError(err)

				onAbortState, err := state.NewDiff(lastAcceptedID, env)
				require.NoError(err)

				executor := ProposalTxExecutor{
					OnCommitState: onCommitState,
					OnAbortState:  onAbortState,
					Backend:       &env.backend,
					Tx:            tx,
				}
				require.NoError(tx.Unsigned.Visit(&executor))

				require.NoError(executor.OnCommitState.Apply(env.state))
			}
			env.state.SetHeight(dummyHeight)
			require.NoError(env.state.Commit())

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
func TestAdvanceTimeTxRemoveSupernetValidator(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()

	supernetID := testSupernet1.ID()
	env.config.TrackedSupernets.Add(supernetID)

	dummyHeight := uint64(1)
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
	env.state.SetHeight(dummyHeight)
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
	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// The above validator is now in the pending staker set

	// Advance time to the first staker's end time.
	env.clk.Set(supernetVdr1EndTime)
	tx, err = newAdvanceTimeTx(t, supernetVdr1EndTime)
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	_, err = executor.OnCommitState.GetCurrentValidator(supernetID, supernetValidatorNodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// Check VM Validators are removed successfully
	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())
	_, ok := env.config.Validators.GetValidator(supernetID, supernetVdr2NodeID)
	require.False(ok)
	_, ok = env.config.Validators.GetValidator(supernetID, supernetValidatorNodeID)
	require.False(ok)
}

func TestTrackedSupernet(t *testing.T) {
	for _, tracked := range []bool{true, false} {
		t.Run(fmt.Sprintf("tracked %t", tracked), func(t *testing.T) {
			require := require.New(t)
			env := newEnvironment(t, apricotPhase5)
			env.ctx.Lock.Lock()
			defer env.ctx.Lock.Unlock()
			dummyHeight := uint64(1)

			supernetID := testSupernet1.ID()
			if tracked {
				env.config.TrackedSupernets.Add(supernetID)
			}

			// Add a supernet validator to the staker set
			supernetValidatorNodeID := genesisNodeIDs[0]

			supernetVdr1StartTime := defaultValidateStartTime.Add(1 * time.Minute)
			supernetVdr1EndTime := defaultValidateStartTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute)
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
			env.state.SetHeight(dummyHeight)
			require.NoError(env.state.Commit())

			// Advance time to the staker's start time.
			env.clk.Set(supernetVdr1StartTime)
			tx, err = newAdvanceTimeTx(t, supernetVdr1StartTime)
			require.NoError(err)

			onCommitState, err := state.NewDiff(lastAcceptedID, env)
			require.NoError(err)

			onAbortState, err := state.NewDiff(lastAcceptedID, env)
			require.NoError(err)

			executor := ProposalTxExecutor{
				OnCommitState: onCommitState,
				OnAbortState:  onAbortState,
				Backend:       &env.backend,
				Tx:            tx,
			}
			require.NoError(tx.Unsigned.Visit(&executor))

			require.NoError(executor.OnCommitState.Apply(env.state))

			env.state.SetHeight(dummyHeight)
			require.NoError(env.state.Commit())
			_, ok := env.config.Validators.GetValidator(supernetID, supernetValidatorNodeID)
			require.True(ok)
		})
	}
}

func TestAdvanceTimeTxDelegatorStakerWeight(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()
	dummyHeight := uint64(1)

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultValidateStartTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMaxStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	_, err := addPendingValidator(
		env,
		pendingValidatorStartTime,
		pendingValidatorEndTime,
		nodeID,
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
	)
	require.NoError(err)

	tx, err := newAdvanceTimeTx(t, pendingValidatorStartTime)
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
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
	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// Advance Time
	tx, err = newAdvanceTimeTx(t, pendingDelegatorStartTime)
	require.NoError(err)

	onCommitState, err = state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err = state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor = ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// Test validator weight after delegation
	vdrWeight = env.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
	require.Equal(env.config.MinDelegatorStake+env.config.MinValidatorStake, vdrWeight)
}

func TestAdvanceTimeTxDelegatorStakers(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()
	dummyHeight := uint64(1)

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultValidateStartTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeID := ids.GenerateTestNodeID()
	_, err := addPendingValidator(env, pendingValidatorStartTime, pendingValidatorEndTime, nodeID, []*secp256k1.PrivateKey{preFundedKeys[0]})
	require.NoError(err)

	tx, err := newAdvanceTimeTx(t, pendingValidatorStartTime)
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// Test validator weight before delegation
	vdrWeight := env.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
	require.Equal(env.config.MinValidatorStake, vdrWeight)

	// Add delegator
	pendingDelegatorStartTime := pendingValidatorStartTime.Add(1 * time.Second)
	pendingDelegatorEndTime := pendingDelegatorStartTime.Add(defaultMinStakingDuration)
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
		[]*secp256k1.PrivateKey{preFundedKeys[0], preFundedKeys[1], preFundedKeys[4]},
	)
	require.NoError(err)

	staker, err := state.NewPendingStaker(
		addDelegatorTx.ID(),
		addDelegatorTx.Unsigned.(*txs.AddDelegatorTx),
	)
	require.NoError(err)

	env.state.PutPendingDelegator(staker)
	env.state.AddTx(addDelegatorTx, status.Committed)
	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// Advance Time
	tx, err = newAdvanceTimeTx(t, pendingDelegatorStartTime)
	require.NoError(err)

	onCommitState, err = state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err = state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor = ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	require.NoError(tx.Unsigned.Visit(&executor))

	require.NoError(executor.OnCommitState.Apply(env.state))

	env.state.SetHeight(dummyHeight)
	require.NoError(env.state.Commit())

	// Test validator weight after delegation
	vdrWeight = env.config.Validators.GetWeight(constants.PrimaryNetworkID, nodeID)
	require.Equal(env.config.MinDelegatorStake+env.config.MinValidatorStake, vdrWeight)
}

func TestAdvanceTimeTxAfterBanff(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, durango)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()
	env.clk.Set(defaultGenesisTime) // VM's clock reads the genesis time
	upgradeTime := env.clk.Time().Add(SyncBound)
	env.config.BanffTime = upgradeTime
	env.config.CortinaTime = upgradeTime
	env.config.DurangoTime = upgradeTime

	// Proposed advancing timestamp to the banff timestamp
	tx, err := newAdvanceTimeTx(t, upgradeTime)
	require.NoError(err)

	onCommitState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	onAbortState, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := ProposalTxExecutor{
		OnCommitState: onCommitState,
		OnAbortState:  onAbortState,
		Backend:       &env.backend,
		Tx:            tx,
	}
	err = tx.Unsigned.Visit(&executor)
	require.ErrorIs(err, ErrAdvanceTimeTxIssuedAfterBanff)
}

// Ensure marshaling/unmarshaling works
func TestAdvanceTimeTxUnmarshal(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(t, apricotPhase5)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()

	chainTime := env.state.GetTimestamp()
	tx, err := newAdvanceTimeTx(t, chainTime.Add(time.Second))
	require.NoError(err)

	bytes, err := txs.Codec.Marshal(txs.CodecVersion, tx)
	require.NoError(err)

	var unmarshaledTx txs.Tx
	_, err = txs.Codec.Unmarshal(bytes, &unmarshaledTx)
	require.NoError(err)

	require.Equal(
		tx.Unsigned.(*txs.AdvanceTimeTx).Time,
		unmarshaledTx.Unsigned.(*txs.AdvanceTimeTx).Time,
	)
}

func addPendingValidator(
	env *environment,
	startTime time.Time,
	endTime time.Time,
	nodeID ids.NodeID,
	keys []*secp256k1.PrivateKey,
) (*txs.Tx, error) {
	addPendingValidatorTx, err := env.txBuilder.NewAddValidatorTx(
		&txs.Validator{
			NodeID: nodeID,
			Start:  uint64(startTime.Unix()),
			End:    uint64(endTime.Unix()),
			Wght:   env.config.MinValidatorStake,
		},
		&secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
		},
		reward.PercentDenominator,
		keys,
	)
	if err != nil {
		return nil, err
	}

	staker, err := state.NewPendingStaker(
		addPendingValidatorTx.ID(),
		addPendingValidatorTx.Unsigned.(*txs.AddValidatorTx),
	)
	if err != nil {
		return nil, err
	}

	env.state.PutPendingValidator(staker)
	env.state.AddTx(addPendingValidatorTx, status.Committed)
	dummyHeight := uint64(1)
	env.state.SetHeight(dummyHeight)
	if err := env.state.Commit(); err != nil {
		return nil, err
	}
	return addPendingValidatorTx, nil
}
