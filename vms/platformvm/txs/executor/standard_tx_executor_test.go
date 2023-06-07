// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/hashing"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/config"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/platformvm/utxo"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

// This tests that the math performed during TransformSupernetTx execution can
// never overflow
const _ time.Duration = math.MaxUint32 * time.Second

var errTest = errors.New("non-nil error")

func TestStandardTxExecutorAddValidatorTxEmptyID(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(false /*=postBanff*/, false /*=postCortina*/)
	env.ctx.Lock.Lock()
	defer func() {
		require.NoError(shutdownEnvironment(env))
	}()

	chainTime := env.state.GetTimestamp()
	startTime := defaultGenesisTime.Add(1 * time.Second)

	tests := []struct {
		banffTime     time.Time
		expectedError error
	}{
		{ // Case: Before banff
			banffTime:     chainTime.Add(1),
			expectedError: errEmptyNodeID,
		},
		{ // Case: At banff
			banffTime:     chainTime,
			expectedError: errEmptyNodeID,
		},
		{ // Case: After banff
			banffTime:     chainTime.Add(-1),
			expectedError: errEmptyNodeID,
		},
	}
	for _, test := range tests {
		// Case: Empty validator node ID after banff
		env.config.BanffTime = test.banffTime

		tx, err := env.txBuilder.NewAddValidatorTx( // create the tx
			env.config.MinValidatorStake,
			uint64(startTime.Unix()),
			uint64(defaultValidateEndTime.Unix()),
			ids.EmptyNodeID,
			ids.GenerateTestShortID(),
			reward.PercentDenominator,
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		stateDiff, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   stateDiff,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.ErrorIs(err, test.expectedError)
	}
}

func TestStandardTxExecutorAddDelegator(t *testing.T) {
	dummyHeight := uint64(1)
	rewardAddress := preFundedKeys[0].PublicKey().Address()
	nodeID := ids.NodeID(rewardAddress)

	newValidatorID := ids.GenerateTestNodeID()
	newValidatorStartTime := uint64(defaultValidateStartTime.Add(5 * time.Second).Unix())
	newValidatorEndTime := uint64(defaultValidateEndTime.Add(-5 * time.Second).Unix())

	// [addMinStakeValidator] adds a new validator to the primary network's
	// pending validator set with the minimum staking amount
	addMinStakeValidator := func(target *environment) {
		tx, err := target.txBuilder.NewAddValidatorTx(
			target.config.MinValidatorStake, // stake amount
			newValidatorStartTime,           // start time
			newValidatorEndTime,             // end time
			newValidatorID,                  // node ID
			rewardAddress,                   // Reward Address
			reward.PercentDenominator,       // Shares
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty,
		)
		require.NoError(t, err)

		staker, err := state.NewCurrentStaker(
			tx.ID(),
			tx.Unsigned.(*txs.AddValidatorTx),
			0,
		)
		require.NoError(t, err)

		target.state.PutCurrentValidator(staker)
		target.state.AddTx(tx, status.Committed)
		target.state.SetHeight(dummyHeight)
		err = target.state.Commit()
		require.NoError(t, err)
	}

	// [addMaxStakeValidator] adds a new validator to the primary network's
	// pending validator set with the maximum staking amount
	addMaxStakeValidator := func(target *environment) {
		tx, err := target.txBuilder.NewAddValidatorTx(
			target.config.MaxValidatorStake, // stake amount
			newValidatorStartTime,           // start time
			newValidatorEndTime,             // end time
			newValidatorID,                  // node ID
			rewardAddress,                   // Reward Address
			reward.PercentDenominator,       // Shared
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty,
		)
		require.NoError(t, err)

		staker, err := state.NewCurrentStaker(
			tx.ID(),
			tx.Unsigned.(*txs.AddValidatorTx),
			0,
		)
		require.NoError(t, err)

		target.state.PutCurrentValidator(staker)
		target.state.AddTx(tx, status.Committed)
		target.state.SetHeight(dummyHeight)
		err = target.state.Commit()
		require.NoError(t, err)
	}

	dummyH := newEnvironment(false /*=postBanff*/, false /*=postCortina*/)
	currentTimestamp := dummyH.state.GetTimestamp()

	type test struct {
		stakeAmount   uint64
		startTime     uint64
		endTime       uint64
		nodeID        ids.NodeID
		rewardAddress ids.ShortID
		feeKeys       []*secp256k1.PrivateKey
		setup         func(*environment)
		AP3Time       time.Time
		shouldErr     bool
		description   string
	}

	tests := []test{
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     uint64(defaultValidateStartTime.Unix()),
			endTime:       uint64(defaultValidateEndTime.Unix()) + 1,
			nodeID:        nodeID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         nil,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "validator stops validating primary network earlier than supernet",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     uint64(currentTimestamp.Add(MaxFutureStartTime + time.Second).Unix()),
			endTime:       uint64(currentTimestamp.Add(MaxFutureStartTime * 2).Unix()),
			nodeID:        nodeID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         nil,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   fmt.Sprintf("validator should not be added more than (%s) in the future", MaxFutureStartTime),
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     uint64(defaultValidateStartTime.Unix()),
			endTime:       uint64(defaultValidateEndTime.Unix()) + 1,
			nodeID:        nodeID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         nil,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "end time is after the primary network end time",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     uint64(defaultValidateStartTime.Add(5 * time.Second).Unix()),
			endTime:       uint64(defaultValidateEndTime.Add(-5 * time.Second).Unix()),
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         nil,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "validator not in the current or pending validator sets of the supernet",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     newValidatorStartTime - 1, // start validating supernet before primary network
			endTime:       newValidatorEndTime,
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         addMinStakeValidator,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "validator starts validating supernet before primary network",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     newValidatorStartTime,
			endTime:       newValidatorEndTime + 1, // stop validating supernet after stopping validating primary network
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         addMinStakeValidator,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "validator stops validating primary network before supernet",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     newValidatorStartTime, // same start time as for primary network
			endTime:       newValidatorEndTime,   // same end time as for primary network
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         addMinStakeValidator,
			AP3Time:       defaultGenesisTime,
			shouldErr:     false,
			description:   "valid",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,           // weight
			startTime:     uint64(currentTimestamp.Unix()),           // start time
			endTime:       uint64(defaultValidateEndTime.Unix()),     // end time
			nodeID:        nodeID,                                    // node ID
			rewardAddress: rewardAddress,                             // Reward Address
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]}, // tx fee payer
			setup:         nil,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "starts validating at current timestamp",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,           // weight
			startTime:     uint64(defaultValidateStartTime.Unix()),   // start time
			endTime:       uint64(defaultValidateEndTime.Unix()),     // end time
			nodeID:        nodeID,                                    // node ID
			rewardAddress: rewardAddress,                             // Reward Address
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[1]}, // tx fee payer
			setup: func(target *environment) { // Remove all UTXOs owned by keys[1]
				utxoIDs, err := target.state.UTXOIDs(
					preFundedKeys[1].PublicKey().Address().Bytes(),
					ids.Empty,
					math.MaxInt32)
				require.NoError(t, err)

				for _, utxoID := range utxoIDs {
					target.state.DeleteUTXO(utxoID)
				}
				target.state.SetHeight(dummyHeight)
				err = target.state.Commit()
				require.NoError(t, err)
			},
			AP3Time:     defaultGenesisTime,
			shouldErr:   true,
			description: "tx fee paying key has no funds",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     newValidatorStartTime, // same start time as for primary network
			endTime:       newValidatorEndTime,   // same end time as for primary network
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         addMaxStakeValidator,
			AP3Time:       defaultValidateEndTime,
			shouldErr:     false,
			description:   "over delegation before AP3",
		},
		{
			stakeAmount:   dummyH.config.MinDelegatorStake,
			startTime:     newValidatorStartTime, // same start time as for primary network
			endTime:       newValidatorEndTime,   // same end time as for primary network
			nodeID:        newValidatorID,
			rewardAddress: rewardAddress,
			feeKeys:       []*secp256k1.PrivateKey{preFundedKeys[0]},
			setup:         addMaxStakeValidator,
			AP3Time:       defaultGenesisTime,
			shouldErr:     true,
			description:   "over delegation after AP3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			require := require.New(t)
			freshTH := newEnvironment(false /*=postBanff*/, false /*=postCortina*/)
			freshTH.config.ApricotPhase3Time = tt.AP3Time
			defer func() {
				require.NoError(shutdownEnvironment(freshTH))
			}()

			tx, err := freshTH.txBuilder.NewAddDelegatorTx(
				tt.stakeAmount,
				tt.startTime,
				tt.endTime,
				tt.nodeID,
				tt.rewardAddress,
				tt.feeKeys,
				ids.ShortEmpty,
			)
			require.NoError(err)

			if tt.setup != nil {
				tt.setup(freshTH)
			}

			onAcceptState, err := state.NewDiff(lastAcceptedID, freshTH)
			require.NoError(err)

			freshTH.config.BanffTime = onAcceptState.GetTimestamp()

			executor := StandardTxExecutor{
				Backend: &freshTH.backend,
				State:   onAcceptState,
				Tx:      tx,
			}
			err = tx.Unsigned.Visit(&executor)
			if tt.shouldErr {
				require.Error(err)
			} else {
				require.NoError(err)
			}

			mempoolExecutor := MempoolTxVerifier{
				Backend:       &freshTH.backend,
				ParentID:      lastAcceptedID,
				StateVersions: freshTH,
				Tx:            tx,
			}
			err = tx.Unsigned.Visit(&mempoolExecutor)
			if tt.shouldErr {
				require.Error(err)
			} else {
				require.NoError(err)
			}
		})
	}
}

func TestStandardTxExecutorAddSupernetValidator(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(false /*=postBanff*/, false /*=postCortina*/)
	env.ctx.Lock.Lock()
	defer func() {
		require.NoError(shutdownEnvironment(env))
	}()

	nodeID := preFundedKeys[0].PublicKey().Address()
	env.config.BanffTime = env.state.GetTimestamp()

	{
		// Case: Proposed validator currently validating primary network
		// but stops validating supernet after stops validating primary network
		// (note that keys[0] is a genesis validator)
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(defaultValidateStartTime.Unix()),
			uint64(defaultValidateEndTime.Unix())+1,
			ids.NodeID(nodeID),
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because validator stops validating primary network earlier than supernet")
	}

	{
		// Case: Proposed validator currently validating primary network
		// and proposed supernet validation period is subset of
		// primary network validation period
		// (note that keys[0] is a genesis validator)
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(defaultValidateStartTime.Unix()+1),
			uint64(defaultValidateEndTime.Unix()),
			ids.NodeID(nodeID),
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.NoError(err)
	}

	// Add a validator to pending validator set of primary network
	key, err := testKeyfactory.NewPrivateKey()
	require.NoError(err)

	pendingDSValidatorID := ids.NodeID(key.PublicKey().Address())

	// starts validating primary network 10 seconds after genesis
	dsStartTime := defaultGenesisTime.Add(10 * time.Second)
	dsEndTime := dsStartTime.Add(5 * defaultMinStakingDuration)

	addDSTx, err := env.txBuilder.NewAddValidatorTx(
		env.config.MinValidatorStake, // stake amount
		uint64(dsStartTime.Unix()),   // start time
		uint64(dsEndTime.Unix()),     // end time
		pendingDSValidatorID,         // node ID
		nodeID,                       // reward address
		reward.PercentDenominator,    // shares
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
		ids.ShortEmpty,
	)
	require.NoError(err)

	{
		// Case: Proposed validator isn't in pending or current validator sets
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(dsStartTime.Unix()), // start validating supernet before primary network
			uint64(dsEndTime.Unix()),
			pendingDSValidatorID,
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because validator not in the current or pending validator sets of the primary network")
	}

	staker, err := state.NewCurrentStaker(
		addDSTx.ID(),
		addDSTx.Unsigned.(*txs.AddValidatorTx),
		0,
	)
	require.NoError(err)

	env.state.PutCurrentValidator(staker)
	env.state.AddTx(addDSTx, status.Committed)
	dummyHeight := uint64(1)
	env.state.SetHeight(dummyHeight)
	err = env.state.Commit()
	require.NoError(err)

	// Node with ID key.PublicKey().Address() now a pending validator for primary network

	{
		// Case: Proposed validator is pending validator of primary network
		// but starts validating supernet before primary network
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(dsStartTime.Unix())-1, // start validating supernet before primary network
			uint64(dsEndTime.Unix()),
			pendingDSValidatorID,
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because validator starts validating primary network before starting to validate primary network")
	}

	{
		// Case: Proposed validator is pending validator of primary network
		// but stops validating supernet after primary network
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(dsStartTime.Unix()),
			uint64(dsEndTime.Unix())+1, // stop validating supernet after stopping validating primary network
			pendingDSValidatorID,
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because validator stops validating primary network after stops validating primary network")
	}

	{
		// Case: Proposed validator is pending validator of primary network and
		// period validating supernet is subset of time validating primary network
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,
			uint64(dsStartTime.Unix()), // same start time as for primary network
			uint64(dsEndTime.Unix()),   // same end time as for primary network
			pendingDSValidatorID,
			testSupernet1.ID(),
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)
		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.NoError(err)
	}

	// Case: Proposed validator start validating at/before current timestamp
	// First, advance the timestamp
	newTimestamp := defaultGenesisTime.Add(2 * time.Second)
	env.state.SetTimestamp(newTimestamp)

	{
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,               // weight
			uint64(newTimestamp.Unix()), // start time
			uint64(newTimestamp.Add(defaultMinStakingDuration).Unix()), // end time
			ids.NodeID(nodeID), // node ID
			testSupernet1.ID(), // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because starts validating at current timestamp")
	}

	// reset the timestamp
	env.state.SetTimestamp(defaultGenesisTime)

	// Case: Proposed validator already validating the supernet
	// First, add validator as validator of supernet
	supernetTx, err := env.txBuilder.NewAddSupernetValidatorTx(
		defaultWeight,                           // weight
		uint64(defaultValidateStartTime.Unix()), // start time
		uint64(defaultValidateEndTime.Unix()),   // end time
		ids.NodeID(nodeID),                      // node ID
		testSupernet1.ID(),                      // supernet ID
		[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
		ids.ShortEmpty,
	)
	require.NoError(err)

	staker, err = state.NewCurrentStaker(
		supernetTx.ID(),
		supernetTx.Unsigned.(*txs.AddSupernetValidatorTx),
		0,
	)
	require.NoError(err)

	env.state.PutCurrentValidator(staker)
	env.state.AddTx(supernetTx, status.Committed)
	env.state.SetHeight(dummyHeight)
	err = env.state.Commit()
	require.NoError(err)

	{
		// Node with ID nodeIDKey.PublicKey().Address() now validating supernet with ID testSupernet1.ID
		duplicateSupernetTx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,                           // weight
			uint64(defaultValidateStartTime.Unix()), // start time
			uint64(defaultValidateEndTime.Unix()),   // end time
			ids.NodeID(nodeID),                      // node ID
			testSupernet1.ID(),                      // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      duplicateSupernetTx,
		}
		err = duplicateSupernetTx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because validator already validating the specified supernet")
	}

	env.state.DeleteCurrentValidator(staker)
	env.state.SetHeight(dummyHeight)
	err = env.state.Commit()
	require.NoError(err)

	{
		// Case: Too many signatures
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,                     // weight
			uint64(defaultGenesisTime.Unix()), // start time
			uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix())+1, // end time
			ids.NodeID(nodeID), // node ID
			testSupernet1.ID(), // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1], testSupernet1ControlKeys[2]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because tx has 3 signatures but only 2 needed")
	}

	{
		// Case: Too few signatures
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,                     // weight
			uint64(defaultGenesisTime.Unix()), // start time
			uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix()), // end time
			ids.NodeID(nodeID), // node ID
			testSupernet1.ID(), // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[2]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		// Remove a signature
		addSupernetValidatorTx := tx.Unsigned.(*txs.AddSupernetValidatorTx)
		input := addSupernetValidatorTx.SupernetAuth.(*secp256k1fx.Input)
		input.SigIndices = input.SigIndices[1:]
		// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
		addSupernetValidatorTx.SyntacticallyVerified = false

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because not enough control sigs")
	}

	{
		// Case: Control Signature from invalid key (keys[3] is not a control key)
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,                     // weight
			uint64(defaultGenesisTime.Unix()), // start time
			uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix()), // end time
			ids.NodeID(nodeID), // node ID
			testSupernet1.ID(), // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], preFundedKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		// Replace a valid signature with one from keys[3]
		sig, err := preFundedKeys[3].SignHash(hashing.ComputeHash256(tx.Unsigned.Bytes()))
		require.NoError(err)
		copy(tx.Creds[0].(*secp256k1fx.Credential).Sigs[0][:], sig)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because a control sig is invalid")
	}

	{
		// Case: Proposed validator in pending validator set for supernet
		// First, add validator to pending validator set of supernet
		tx, err := env.txBuilder.NewAddSupernetValidatorTx(
			defaultWeight,                       // weight
			uint64(defaultGenesisTime.Unix())+1, // start time
			uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix())+1, // end time
			ids.NodeID(nodeID), // node ID
			testSupernet1.ID(), // supernet ID
			[]*secp256k1.PrivateKey{testSupernet1ControlKeys[0], testSupernet1ControlKeys[1]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		staker, err = state.NewCurrentStaker(
			supernetTx.ID(),
			supernetTx.Unsigned.(*txs.AddSupernetValidatorTx),
			0,
		)
		require.NoError(err)

		env.state.PutCurrentValidator(staker)
		env.state.AddTx(tx, status.Committed)
		env.state.SetHeight(dummyHeight)
		err = env.state.Commit()
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed verification because validator already in pending validator set of the specified supernet")
	}
}

func TestStandardTxExecutorAddValidator(t *testing.T) {
	require := require.New(t)
	env := newEnvironment(false /*=postBanff*/, false /*=postCortina*/)
	env.ctx.Lock.Lock()
	defer func() {
		require.NoError(shutdownEnvironment(env))
	}()

	nodeID := ids.GenerateTestNodeID()

	env.config.BanffTime = env.state.GetTimestamp()

	{
		// Case: Validator's start time too early
		tx, err := env.txBuilder.NewAddValidatorTx(
			env.config.MinValidatorStake,
			uint64(defaultValidateStartTime.Unix())-1,
			uint64(defaultValidateEndTime.Unix()),
			nodeID,
			ids.ShortEmpty,
			reward.PercentDenominator,
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should've errored because start time too early")
	}

	{
		// Case: Validator's start time too far in the future
		tx, err := env.txBuilder.NewAddValidatorTx(
			env.config.MinValidatorStake,
			uint64(defaultValidateStartTime.Add(MaxFutureStartTime).Unix()+1),
			uint64(defaultValidateStartTime.Add(MaxFutureStartTime).Add(defaultMinStakingDuration).Unix()+1),
			nodeID,
			ids.ShortEmpty,
			reward.PercentDenominator,
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should've errored because start time too far in the future")
	}

	{
		// Case: Validator already validating primary network
		tx, err := env.txBuilder.NewAddValidatorTx(
			env.config.MinValidatorStake,
			uint64(defaultValidateStartTime.Unix()),
			uint64(defaultValidateEndTime.Unix()),
			nodeID,
			ids.ShortEmpty,
			reward.PercentDenominator,
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should've errored because validator already validating")
	}

	{
		// Case: Validator in pending validator set of primary network
		startTime := defaultGenesisTime.Add(1 * time.Second)
		tx, err := env.txBuilder.NewAddValidatorTx(
			env.config.MinValidatorStake,                            // stake amount
			uint64(startTime.Unix()),                                // start time
			uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
			nodeID,
			ids.ShortEmpty,
			reward.PercentDenominator, // shares
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr // key
		)
		require.NoError(err)

		staker, err := state.NewCurrentStaker(
			tx.ID(),
			tx.Unsigned.(*txs.AddValidatorTx),
			0,
		)
		require.NoError(err)

		env.state.PutCurrentValidator(staker)
		env.state.AddTx(tx, status.Committed)
		dummyHeight := uint64(1)
		env.state.SetHeight(dummyHeight)
		err = env.state.Commit()
		require.NoError(err)

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because validator in pending validator set")
	}

	{
		// Case: Validator doesn't have enough tokens to cover stake amount
		tx, err := env.txBuilder.NewAddValidatorTx( // create the tx
			env.config.MinValidatorStake,
			uint64(defaultValidateStartTime.Unix()),
			uint64(defaultValidateEndTime.Unix()),
			nodeID,
			ids.ShortEmpty,
			reward.PercentDenominator,
			[]*secp256k1.PrivateKey{preFundedKeys[0]},
			ids.ShortEmpty, // change addr
		)
		require.NoError(err)

		// Remove all UTXOs owned by preFundedKeys[0]
		utxoIDs, err := env.state.UTXOIDs(preFundedKeys[0].PublicKey().Address().Bytes(), ids.Empty, math.MaxInt32)
		require.NoError(err)

		for _, utxoID := range utxoIDs {
			env.state.DeleteUTXO(utxoID)
		}

		onAcceptState, err := state.NewDiff(lastAcceptedID, env)
		require.NoError(err)

		executor := StandardTxExecutor{
			Backend: &env.backend,
			State:   onAcceptState,
			Tx:      tx,
		}
		err = tx.Unsigned.Visit(&executor)
		require.Error(err, "should have failed because tx fee paying key has no funds")
	}
}

// Returns a RemoveSupernetValidatorTx that passes syntactic verification.
func newRemoveSupernetValidatorTx(t *testing.T) (*txs.RemoveSupernetValidatorTx, *txs.Tx) {
	t.Helper()

	creds := []verify.Verifiable{
		&secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
		&secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
	}
	unsignedTx := &txs.RemoveSupernetValidatorTx{
		BaseTx: txs.BaseTx{
			BaseTx: avax.BaseTx{
				Ins: []*avax.TransferableInput{{
					UTXOID: avax.UTXOID{
						TxID: ids.GenerateTestID(),
					},
					Asset: avax.Asset{
						ID: ids.GenerateTestID(),
					},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 1},
						},
					},
				}},
				Outs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{
							ID: ids.GenerateTestID(),
						},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
							},
						},
					},
				},
				Memo: []byte("hi"),
			},
		},
		Supernet: ids.GenerateTestID(),
		NodeID:   ids.GenerateTestNodeID(),
		SupernetAuth: &secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
	}
	tx := &txs.Tx{
		Unsigned: unsignedTx,
		Creds:    creds,
	}
	require.NoError(t, tx.Initialize(txs.Codec))
	return unsignedTx, tx
}

// mock implementations that can be used in tests
// for verifying RemoveSupernetValidatorTx.
type removeSupernetValidatorTxVerifyEnv struct {
	banffTime   time.Time
	fx          *fx.MockFx
	flowChecker *utxo.MockVerifier
	unsignedTx  *txs.RemoveSupernetValidatorTx
	tx          *txs.Tx
	state       *state.MockDiff
	staker      *state.Staker
}

// Returns mock implementations that can be used in tests
// for verifying RemoveSupernetValidatorTx.
func newValidRemoveSupernetValidatorTxVerifyEnv(t *testing.T, ctrl *gomock.Controller) removeSupernetValidatorTxVerifyEnv {
	t.Helper()

	now := time.Now()
	mockFx := fx.NewMockFx(ctrl)
	mockFlowChecker := utxo.NewMockVerifier(ctrl)
	unsignedTx, tx := newRemoveSupernetValidatorTx(t)
	mockState := state.NewMockDiff(ctrl)
	return removeSupernetValidatorTxVerifyEnv{
		banffTime:   now,
		fx:          mockFx,
		flowChecker: mockFlowChecker,
		unsignedTx:  unsignedTx,
		tx:          tx,
		state:       mockState,
		staker: &state.Staker{
			TxID:     ids.GenerateTestID(),
			NodeID:   ids.GenerateTestNodeID(),
			Priority: txs.SupernetPermissionedValidatorCurrentPriority,
		},
	}
}

func TestStandardExecutorRemoveSupernetValidatorTx(t *testing.T) {
	type test struct {
		name        string
		newExecutor func(*gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor)
		shouldErr   bool
		expectedErr error
	}

	tests := []test{
		{
			name: "valid tx",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)

				// Set dependency expectations.
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(env.staker, nil).Times(1)
				supernetOwner := fx.NewMockOwner(ctrl)
				supernetTx := &txs.Tx{
					Unsigned: &txs.CreateSupernetTx{
						Owner: supernetOwner,
					},
				}
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(supernetTx, status.Committed, nil).Times(1)
				env.fx.EXPECT().VerifyPermission(env.unsignedTx, env.unsignedTx.SupernetAuth, env.tx.Creds[len(env.tx.Creds)-1], supernetOwner).Return(nil).Times(1)
				env.flowChecker.EXPECT().VerifySpend(
					env.unsignedTx, env.state, env.unsignedTx.Ins, env.unsignedTx.Outs, env.tx.Creds[:len(env.tx.Creds)-1], gomock.Any(),
				).Return(nil).Times(1)
				env.state.EXPECT().DeleteCurrentValidator(env.staker)
				env.state.EXPECT().DeleteUTXO(gomock.Any()).Times(len(env.unsignedTx.Ins))
				env.state.EXPECT().AddUTXO(gomock.Any()).Times(len(env.unsignedTx.Outs))
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr: false,
		},
		{
			name: "tx fails syntactic verification",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				// Setting the supernet ID to the Primary Network ID makes the tx fail syntactic verification
				env.tx.Unsigned.(*txs.RemoveSupernetValidatorTx).Supernet = constants.PrimaryNetworkID
				env.state = state.NewMockDiff(ctrl)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr: true,
		},
		{
			name: "node isn't a validator of the supernet",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				env.state = state.NewMockDiff(ctrl)
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(nil, database.ErrNotFound)
				env.state.EXPECT().GetPendingValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(nil, database.ErrNotFound)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errNotValidator,
		},
		{
			name: "validator is permissionless",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)

				staker := *env.staker
				staker.Priority = txs.SupernetPermissionlessValidatorCurrentPriority

				// Set dependency expectations.
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(&staker, nil).Times(1)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errRemovePermissionlessValidator,
		},
		{
			name: "tx has no credentials",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				// Remove credentials
				env.tx.Creds = nil
				env.state = state.NewMockDiff(ctrl)
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(env.staker, nil)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errWrongNumberOfCredentials,
		},
		{
			name: "can't find supernet",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				env.state = state.NewMockDiff(ctrl)
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(env.staker, nil)
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(nil, status.Unknown, database.ErrNotFound)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errCantFindSupernet,
		},
		{
			name: "no permission to remove validator",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				env.state = state.NewMockDiff(ctrl)
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(env.staker, nil)
				supernetOwner := fx.NewMockOwner(ctrl)
				supernetTx := &txs.Tx{
					Unsigned: &txs.CreateSupernetTx{
						Owner: supernetOwner,
					},
				}
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(supernetTx, status.Committed, nil)
				env.fx.EXPECT().VerifyPermission(gomock.Any(), env.unsignedTx.SupernetAuth, env.tx.Creds[len(env.tx.Creds)-1], supernetOwner).Return(errTest)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errUnauthorizedSupernetModification,
		},
		{
			name: "flow checker failed",
			newExecutor: func(ctrl *gomock.Controller) (*txs.RemoveSupernetValidatorTx, *StandardTxExecutor) {
				env := newValidRemoveSupernetValidatorTxVerifyEnv(t, ctrl)
				env.state = state.NewMockDiff(ctrl)
				env.state.EXPECT().GetCurrentValidator(env.unsignedTx.Supernet, env.unsignedTx.NodeID).Return(env.staker, nil)
				supernetOwner := fx.NewMockOwner(ctrl)
				supernetTx := &txs.Tx{
					Unsigned: &txs.CreateSupernetTx{
						Owner: supernetOwner,
					},
				}
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(supernetTx, status.Committed, nil)
				env.fx.EXPECT().VerifyPermission(gomock.Any(), env.unsignedTx.SupernetAuth, env.tx.Creds[len(env.tx.Creds)-1], supernetOwner).Return(nil)
				env.flowChecker.EXPECT().VerifySpend(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(errTest)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			shouldErr:   true,
			expectedErr: errFlowCheckFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			unsignedTx, executor := tt.newExecutor(ctrl)
			err := executor.RemoveSupernetValidatorTx(unsignedTx)
			if tt.shouldErr {
				require.Error(err)
				if tt.expectedErr != nil {
					require.ErrorIs(err, tt.expectedErr)
				}
				return
			}
			require.NoError(err)
		})
	}
}

// Returns a TransformSupernetTx that passes syntactic verification.
func newTransformSupernetTx(t *testing.T) (*txs.TransformSupernetTx, *txs.Tx) {
	t.Helper()

	creds := []verify.Verifiable{
		&secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
		&secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
	}
	unsignedTx := &txs.TransformSupernetTx{
		BaseTx: txs.BaseTx{
			BaseTx: avax.BaseTx{
				Ins: []*avax.TransferableInput{{
					UTXOID: avax.UTXOID{
						TxID: ids.GenerateTestID(),
					},
					Asset: avax.Asset{
						ID: ids.GenerateTestID(),
					},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 1},
						},
					},
				}},
				Outs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{
							ID: ids.GenerateTestID(),
						},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
							OutputOwners: secp256k1fx.OutputOwners{
								Threshold: 1,
								Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
							},
						},
					},
				},
				Memo: []byte("hi"),
			},
		},
		Supernet:                 ids.GenerateTestID(),
		AssetID:                  ids.GenerateTestID(),
		InitialSupply:            10,
		MaximumSupply:            10,
		MinConsumptionRate:       0,
		MaxConsumptionRate:       reward.PercentDenominator,
		MinValidatorStake:        2,
		MaxValidatorStake:        10,
		MinStakeDuration:         1,
		MaxStakeDuration:         2,
		MinDelegationFee:         reward.PercentDenominator,
		MinDelegatorStake:        1,
		MaxValidatorWeightFactor: 1,
		UptimeRequirement:        reward.PercentDenominator,
		SupernetAuth: &secp256k1fx.Credential{
			Sigs: make([][65]byte, 1),
		},
	}
	tx := &txs.Tx{
		Unsigned: unsignedTx,
		Creds:    creds,
	}
	require.NoError(t, tx.Initialize(txs.Codec))
	return unsignedTx, tx
}

// mock implementations that can be used in tests
// for verifying TransformSupernetTx.
type transformSupernetTxVerifyEnv struct {
	banffTime   time.Time
	fx          *fx.MockFx
	flowChecker *utxo.MockVerifier
	unsignedTx  *txs.TransformSupernetTx
	tx          *txs.Tx
	state       *state.MockDiff
	staker      *state.Staker
}

// Returns mock implementations that can be used in tests
// for verifying TransformSupernetTx.
func newValidTransformSupernetTxVerifyEnv(t *testing.T, ctrl *gomock.Controller) transformSupernetTxVerifyEnv {
	t.Helper()

	now := time.Now()
	mockFx := fx.NewMockFx(ctrl)
	mockFlowChecker := utxo.NewMockVerifier(ctrl)
	unsignedTx, tx := newTransformSupernetTx(t)
	mockState := state.NewMockDiff(ctrl)
	return transformSupernetTxVerifyEnv{
		banffTime:   now,
		fx:          mockFx,
		flowChecker: mockFlowChecker,
		unsignedTx:  unsignedTx,
		tx:          tx,
		state:       mockState,
		staker: &state.Staker{
			TxID:   ids.GenerateTestID(),
			NodeID: ids.GenerateTestNodeID(),
		},
	}
}

func TestStandardExecutorTransformSupernetTx(t *testing.T) {
	type test struct {
		name        string
		newExecutor func(*gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor)
		err         error
	}

	tests := []test{
		{
			name: "tx fails syntactic verification",
			newExecutor: func(ctrl *gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor) {
				env := newValidTransformSupernetTxVerifyEnv(t, ctrl)
				// Setting the tx to nil makes the tx fail syntactic verification
				env.tx.Unsigned = (*txs.TransformSupernetTx)(nil)
				env.state = state.NewMockDiff(ctrl)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			err: txs.ErrNilTx,
		},
		{
			name: "max stake duration too large",
			newExecutor: func(ctrl *gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor) {
				env := newValidTransformSupernetTxVerifyEnv(t, ctrl)
				env.unsignedTx.MaxStakeDuration = math.MaxUint32
				env.state = state.NewMockDiff(ctrl)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime: env.banffTime,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			err: errMaxStakeDurationTooLarge,
		},
		{
			name: "fail supernet authorization",
			newExecutor: func(ctrl *gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor) {
				env := newValidTransformSupernetTxVerifyEnv(t, ctrl)
				// Remove credentials
				env.tx.Creds = nil
				env.state = state.NewMockDiff(ctrl)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime:        env.banffTime,
							MaxStakeDuration: math.MaxInt64,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			err: errWrongNumberOfCredentials,
		},
		{
			name: "flow checker failed",
			newExecutor: func(ctrl *gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor) {
				env := newValidTransformSupernetTxVerifyEnv(t, ctrl)
				env.state = state.NewMockDiff(ctrl)
				supernetOwner := fx.NewMockOwner(ctrl)
				supernetTx := &txs.Tx{
					Unsigned: &txs.CreateSupernetTx{
						Owner: supernetOwner,
					},
				}
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(supernetTx, status.Committed, nil)
				env.state.EXPECT().GetSupernetTransformation(env.unsignedTx.Supernet).Return(nil, database.ErrNotFound).Times(1)
				env.fx.EXPECT().VerifyPermission(gomock.Any(), env.unsignedTx.SupernetAuth, env.tx.Creds[len(env.tx.Creds)-1], supernetOwner).Return(nil)
				env.flowChecker.EXPECT().VerifySpend(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return(errFlowCheckFailed)
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime:        env.banffTime,
							MaxStakeDuration: math.MaxInt64,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			err: errFlowCheckFailed,
		},
		{
			name: "valid tx",
			newExecutor: func(ctrl *gomock.Controller) (*txs.TransformSupernetTx, *StandardTxExecutor) {
				env := newValidTransformSupernetTxVerifyEnv(t, ctrl)

				// Set dependency expectations.
				supernetOwner := fx.NewMockOwner(ctrl)
				supernetTx := &txs.Tx{
					Unsigned: &txs.CreateSupernetTx{
						Owner: supernetOwner,
					},
				}
				env.state.EXPECT().GetTx(env.unsignedTx.Supernet).Return(supernetTx, status.Committed, nil).Times(1)
				env.state.EXPECT().GetSupernetTransformation(env.unsignedTx.Supernet).Return(nil, database.ErrNotFound).Times(1)
				env.fx.EXPECT().VerifyPermission(env.unsignedTx, env.unsignedTx.SupernetAuth, env.tx.Creds[len(env.tx.Creds)-1], supernetOwner).Return(nil).Times(1)
				env.flowChecker.EXPECT().VerifySpend(
					env.unsignedTx, env.state, env.unsignedTx.Ins, env.unsignedTx.Outs, env.tx.Creds[:len(env.tx.Creds)-1], gomock.Any(),
				).Return(nil).Times(1)
				env.state.EXPECT().AddSupernetTransformation(env.tx)
				env.state.EXPECT().SetCurrentSupply(env.unsignedTx.Supernet, env.unsignedTx.InitialSupply)
				env.state.EXPECT().DeleteUTXO(gomock.Any()).Times(len(env.unsignedTx.Ins))
				env.state.EXPECT().AddUTXO(gomock.Any()).Times(len(env.unsignedTx.Outs))
				e := &StandardTxExecutor{
					Backend: &Backend{
						Config: &config.Config{
							BanffTime:        env.banffTime,
							MaxStakeDuration: math.MaxInt64,
						},
						Bootstrapped: &utils.Atomic[bool]{},
						Fx:           env.fx,
						FlowChecker:  env.flowChecker,
						Ctx:          &snow.Context{},
					},
					Tx:    env.tx,
					State: env.state,
				}
				e.Bootstrapped.Set(true)
				return env.unsignedTx, e
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			unsignedTx, executor := tt.newExecutor(ctrl)
			err := executor.TransformSupernetTx(unsignedTx)
			require.ErrorIs(t, err, tt.err)
		})
	}
}
