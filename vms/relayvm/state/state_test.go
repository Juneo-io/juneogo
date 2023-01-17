// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/blocks"
	"github.com/Juneo-io/juneogo/vms/relayvm/config"
	"github.com/Juneo-io/juneogo/vms/relayvm/genesis"
	"github.com/Juneo-io/juneogo/vms/relayvm/metrics"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/relayvm/validator"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	initialTxID             = ids.GenerateTestID()
	initialNodeID           = ids.GenerateTestNodeID()
	initialTime             = time.Now().Round(time.Second)
	initialValidatorEndTime = initialTime.Add(28 * 24 * time.Hour)
)

func TestStateInitialization(t *testing.T) {
	require := require.New(t)
	s, db := newUninitializedState(require)

	shouldInit, err := s.(*state).shouldInit()
	require.NoError(err)
	require.True(shouldInit)

	require.NoError(s.(*state).doneInit())
	require.NoError(s.Commit())

	s = newStateFromDB(require, db)

	shouldInit, err = s.(*state).shouldInit()
	require.NoError(err)
	require.False(shouldInit)
}

func TestStateSyncGenesis(t *testing.T) {
	require := require.New(t)
	state, _ := newInitializedState(require)

	staker, err := state.GetCurrentValidator(constants.PrimaryNetworkID, initialNodeID)
	require.NoError(err)
	require.NotNil(staker)
	require.Equal(initialNodeID, staker.NodeID)

	delegatorIterator, err := state.GetCurrentDelegatorIterator(constants.PrimaryNetworkID, initialNodeID)
	require.NoError(err)
	assertIteratorsEqual(t, EmptyIterator, delegatorIterator)

	stakerIterator, err := state.GetCurrentStakerIterator()
	require.NoError(err)
	assertIteratorsEqual(t, NewSliceIterator(staker), stakerIterator)

	_, err = state.GetPendingValidator(constants.PrimaryNetworkID, initialNodeID)
	require.ErrorIs(err, database.ErrNotFound)

	delegatorIterator, err = state.GetPendingDelegatorIterator(constants.PrimaryNetworkID, initialNodeID)
	require.NoError(err)
	assertIteratorsEqual(t, EmptyIterator, delegatorIterator)
}

func TestGetValidatorWeightDiffs(t *testing.T) {
	require := require.New(t)
	stateIntf, _ := newInitializedState(require)
	state := stateIntf.(*state)

	txID0 := ids.GenerateTestID()
	txID1 := ids.GenerateTestID()
	txID2 := ids.GenerateTestID()
	txID3 := ids.GenerateTestID()

	nodeID0 := ids.GenerateTestNodeID()

	supernetID0 := ids.GenerateTestID()

	type stakerDiff struct {
		validatorsToAdd    []*Staker
		delegatorsToAdd    []*Staker
		validatorsToRemove []*Staker
		delegatorsToRemove []*Staker

		expectedValidatorWeightDiffs map[ids.ID]map[ids.NodeID]*ValidatorWeightDiff
	}
	stakerDiffs := []*stakerDiff{
		{
			validatorsToAdd: []*Staker{
				{
					TxID:       txID0,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     1,
				},
			},
			expectedValidatorWeightDiffs: map[ids.ID]map[ids.NodeID]*ValidatorWeightDiff{
				constants.PrimaryNetworkID: {
					nodeID0: {
						Decrease: false,
						Amount:   1,
					},
				},
			},
		},
		{
			validatorsToAdd: []*Staker{
				{
					TxID:       txID3,
					NodeID:     nodeID0,
					SupernetID: supernetID0,
					Weight:     10,
				},
			},
			delegatorsToAdd: []*Staker{
				{
					TxID:       txID1,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     5,
				},
			},
			expectedValidatorWeightDiffs: map[ids.ID]map[ids.NodeID]*ValidatorWeightDiff{
				constants.PrimaryNetworkID: {
					nodeID0: {
						Decrease: false,
						Amount:   5,
					},
				},
				supernetID0: {
					nodeID0: {
						Decrease: false,
						Amount:   10,
					},
				},
			},
		},
		{
			delegatorsToAdd: []*Staker{
				{
					TxID:       txID2,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     15,
				},
			},
			delegatorsToRemove: []*Staker{
				{
					TxID:       txID1,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     5,
				},
			},
			expectedValidatorWeightDiffs: map[ids.ID]map[ids.NodeID]*ValidatorWeightDiff{
				constants.PrimaryNetworkID: {
					nodeID0: {
						Decrease: false,
						Amount:   10,
					},
				},
			},
		},
		{
			validatorsToRemove: []*Staker{
				{
					TxID:       txID0,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     1,
				},
				{
					TxID:       txID3,
					NodeID:     nodeID0,
					SupernetID: supernetID0,
					Weight:     10,
				},
			},
			delegatorsToRemove: []*Staker{
				{
					TxID:       txID2,
					NodeID:     nodeID0,
					SupernetID: constants.PrimaryNetworkID,
					Weight:     15,
				},
			},
			expectedValidatorWeightDiffs: map[ids.ID]map[ids.NodeID]*ValidatorWeightDiff{
				constants.PrimaryNetworkID: {
					nodeID0: {
						Decrease: true,
						Amount:   16,
					},
				},
				supernetID0: {
					nodeID0: {
						Decrease: true,
						Amount:   10,
					},
				},
			},
		},
		{},
	}

	for i, stakerDiff := range stakerDiffs {
		for _, validator := range stakerDiff.validatorsToAdd {
			state.PutCurrentValidator(validator)
		}
		for _, delegator := range stakerDiff.delegatorsToAdd {
			state.PutCurrentDelegator(delegator)
		}
		for _, validator := range stakerDiff.validatorsToRemove {
			state.DeleteCurrentValidator(validator)
		}
		for _, delegator := range stakerDiff.delegatorsToRemove {
			state.DeleteCurrentDelegator(delegator)
		}
		state.SetHeight(uint64(i + 1))
		require.NoError(state.Commit())

		// Calling write again should not change the state.
		state.SetHeight(uint64(i + 1))
		require.NoError(state.Commit())

		for j, stakerDiff := range stakerDiffs[:i+1] {
			for supernetID, expectedValidatorWeightDiffs := range stakerDiff.expectedValidatorWeightDiffs {
				validatorWeightDiffs, err := state.GetValidatorWeightDiffs(uint64(j+1), supernetID)
				require.NoError(err)
				require.Equal(expectedValidatorWeightDiffs, validatorWeightDiffs)
			}

			state.validatorWeightDiffsCache.Flush()
		}
	}
}

func TestGetValidatorPublicKeyDiffs(t *testing.T) {
	require := require.New(t)
	stateIntf, _ := newInitializedState(require)
	state := stateIntf.(*state)

	var (
		numNodes = 6
		txIDs    = make([]ids.ID, numNodes)
		nodeIDs  = make([]ids.NodeID, numNodes)
		sks      = make([]*bls.SecretKey, numNodes)
		pks      = make([]*bls.PublicKey, numNodes)
		pkBytes  = make([][]byte, numNodes)
		err      error
	)
	for i := 0; i < numNodes; i++ {
		txIDs[i] = ids.GenerateTestID()
		nodeIDs[i] = ids.GenerateTestNodeID()
		sks[i], err = bls.NewSecretKey()
		require.NoError(err)
		pks[i] = bls.PublicFromSecretKey(sks[i])
		pkBytes[i] = bls.PublicKeyToBytes(pks[i])
	}

	type stakerDiff struct {
		validatorsToAdd        []*Staker
		validatorsToRemove     []*Staker
		expectedPublicKeyDiffs map[ids.NodeID]*bls.PublicKey
	}
	stakerDiffs := []*stakerDiff{
		{
			// Add two validators
			validatorsToAdd: []*Staker{
				{
					TxID:      txIDs[0],
					NodeID:    nodeIDs[0],
					Weight:    1,
					PublicKey: pks[0],
				},
				{
					TxID:      txIDs[1],
					NodeID:    nodeIDs[1],
					Weight:    10,
					PublicKey: pks[1],
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{},
		},
		{
			// Remove a validator
			validatorsToRemove: []*Staker{
				{
					TxID:      txIDs[0],
					NodeID:    nodeIDs[0],
					Weight:    1,
					PublicKey: pks[0],
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{
				nodeIDs[0]: pks[0],
			},
		},
		{
			// Add 2 validators and remove a validator
			validatorsToAdd: []*Staker{
				{
					TxID:      txIDs[2],
					NodeID:    nodeIDs[2],
					Weight:    10,
					PublicKey: pks[2],
				},
				{
					TxID:      txIDs[3],
					NodeID:    nodeIDs[3],
					Weight:    10,
					PublicKey: pks[3],
				},
			},
			validatorsToRemove: []*Staker{
				{
					TxID:      txIDs[1],
					NodeID:    nodeIDs[1],
					Weight:    10,
					PublicKey: pks[1],
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{
				nodeIDs[1]: pks[1],
			},
		},
		{
			// Remove 2 validators and add a validator
			validatorsToAdd: []*Staker{
				{
					TxID:      txIDs[4],
					NodeID:    nodeIDs[4],
					Weight:    10,
					PublicKey: pks[4],
				},
			},
			validatorsToRemove: []*Staker{
				{
					TxID:      txIDs[2],
					NodeID:    nodeIDs[2],
					Weight:    10,
					PublicKey: pks[2],
				},
				{
					TxID:      txIDs[3],
					NodeID:    nodeIDs[3],
					Weight:    10,
					PublicKey: pks[3],
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{
				nodeIDs[2]: pks[2],
				nodeIDs[3]: pks[3],
			},
		},
		{
			// Add a validator with no pub key
			validatorsToAdd: []*Staker{
				{
					TxID:      txIDs[5],
					NodeID:    nodeIDs[5],
					Weight:    10,
					PublicKey: nil,
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{},
		},
		{
			// Remove a validator with no pub key
			validatorsToRemove: []*Staker{
				{
					TxID:      txIDs[5],
					NodeID:    nodeIDs[5],
					Weight:    10,
					PublicKey: nil,
				},
			},
			expectedPublicKeyDiffs: map[ids.NodeID]*bls.PublicKey{},
		},
	}

	for i, stakerDiff := range stakerDiffs {
		for _, validator := range stakerDiff.validatorsToAdd {
			state.PutCurrentValidator(validator)
		}
		for _, validator := range stakerDiff.validatorsToRemove {
			state.DeleteCurrentValidator(validator)
		}
		state.SetHeight(uint64(i + 1))
		require.NoError(state.Commit())

		// Calling write again should not change the state.
		state.SetHeight(uint64(i + 1))
		require.NoError(state.Commit())

		for j, stakerDiff := range stakerDiffs[:i+1] {
			pkDiffs, err := state.GetValidatorPublicKeyDiffs(uint64(j + 1))
			require.NoError(err)
			require.Equal(stakerDiff.expectedPublicKeyDiffs, pkDiffs)
			state.validatorPublicKeyDiffsCache.Flush()
		}
	}
}

func newInitializedState(require *require.Assertions) (State, database.Database) {
	s, db := newUninitializedState(require)

	initialValidator := &txs.AddValidatorTx{
		Validator: validator.Validator{
			NodeID: initialNodeID,
			Start:  uint64(initialTime.Unix()),
			End:    uint64(initialValidatorEndTime.Unix()),
			Wght:   units.June,
		},
		StakeOuts: []*june.TransferableOutput{
			{
				Asset: june.Asset{ID: initialTxID},
				Out: &secp256k1fx.TransferOutput{
					Amt: units.June,
				},
			},
		},
		RewardsOwner:     &secp256k1fx.OutputOwners{},
		DelegationShares: reward.PercentDenominator,
	}
	initialValidatorTx := &txs.Tx{Unsigned: initialValidator}
	require.NoError(initialValidatorTx.Initialize(txs.Codec))

	initialChain := &txs.CreateChainTx{
		SupernetID:   constants.PrimaryNetworkID,
		ChainName:    "x",
		VMID:         constants.JVMID,
		SupernetAuth: &secp256k1fx.Input{},
	}
	initialChainTx := &txs.Tx{Unsigned: initialChain}
	require.NoError(initialChainTx.Initialize(txs.Codec))

	genesisBlkID := ids.GenerateTestID()
	genesisState := &genesis.State{
		UTXOs: []*june.UTXO{
			{
				UTXOID: june.UTXOID{
					TxID:        initialTxID,
					OutputIndex: 0,
				},
				Asset: june.Asset{ID: initialTxID},
				Out: &secp256k1fx.TransferOutput{
					Amt: units.Schmeckle,
				},
			},
		},
		Validators: []*txs.Tx{
			initialValidatorTx,
		},
		Chains: []*txs.Tx{
			initialChainTx,
		},
		Timestamp:     uint64(initialTime.Unix()),
		InitialSupply: units.Schmeckle + units.June,
	}

	genesisBlk, err := blocks.NewApricotCommitBlock(genesisBlkID, 0)
	require.NoError(err)
	require.NoError(s.(*state).syncGenesis(genesisBlk, genesisState))

	return s, db
}

func newUninitializedState(require *require.Assertions) (State, database.Database) {
	db := memdb.New()
	return newStateFromDB(require, db), db
}

func newStateFromDB(require *require.Assertions, db database.Database) State {
	vdrs := validators.NewManager()
	primaryVdrs := validators.NewSet()
	_ = vdrs.Add(constants.PrimaryNetworkID, primaryVdrs)
	state, err := new(
		db,
		metrics.Noop,
		&config.Config{
			Validators: vdrs,
		},
		&snow.Context{},
		prometheus.NewRegistry(),
		reward.NewCalculator(reward.Config{
			MintingPeriod: 365 * 24 * time.Hour,
			RewardShare:   50000,
		}),
	)
	require.NoError(err)
	require.NotNil(state)
	return state
}

func TestValidatorWeightDiff(t *testing.T) {
	type test struct {
		name      string
		ops       []func(*ValidatorWeightDiff) error
		shouldErr bool
		expected  ValidatorWeightDiff
	}

	tests := []test{
		{
			name:      "no ops",
			ops:       []func(*ValidatorWeightDiff) error{},
			shouldErr: false,
			expected:  ValidatorWeightDiff{},
		},
		{
			name: "simple decrease",
			ops: []func(*ValidatorWeightDiff) error{
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 1)
				},
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 1)
				},
			},
			shouldErr: false,
			expected: ValidatorWeightDiff{
				Decrease: true,
				Amount:   2,
			},
		},
		{
			name: "decrease overflow",
			ops: []func(*ValidatorWeightDiff) error{
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, math.MaxUint64)
				},
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 1)
				},
			},
			shouldErr: true,
			expected:  ValidatorWeightDiff{},
		},
		{
			name: "simple increase",
			ops: []func(*ValidatorWeightDiff) error{
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 1)
				},
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 1)
				},
			},
			shouldErr: false,
			expected: ValidatorWeightDiff{
				Decrease: false,
				Amount:   2,
			},
		},
		{
			name: "increase overflow",
			ops: []func(*ValidatorWeightDiff) error{
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, math.MaxUint64)
				},
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 1)
				},
			},
			shouldErr: true,
			expected:  ValidatorWeightDiff{},
		},
		{
			name: "varied use",
			ops: []func(*ValidatorWeightDiff) error{
				// Add to 0
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 2) // Value 2
				},
				// Subtract from positive number
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 1) // Value 1
				},
				// Subtract from positive number
				// to make it negative
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 3) // Value -2
				},
				// Subtract from a negative number
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 3) // Value -5
				},
				// Add to a negative number
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 1) // Value -4
				},
				// Add to a negative number
				// to make it positive
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 5) // Value 1
				},
				// Add to a positive number
				func(d *ValidatorWeightDiff) error {
					return d.Add(false, 1) // Value 2
				},
				// Get to zero
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 2) // Value 0
				},
				// Subtract from zero
				func(d *ValidatorWeightDiff) error {
					return d.Add(true, 2) // Value -2
				},
			},
			shouldErr: false,
			expected: ValidatorWeightDiff{
				Decrease: true,
				Amount:   2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			diff := &ValidatorWeightDiff{}
			errs := wrappers.Errs{}
			for _, op := range tt.ops {
				errs.Add(op(diff))
			}
			if tt.shouldErr {
				require.Error(errs.Err)
				return
			}
			require.NoError(errs.Err)
			require.Equal(tt.expected, *diff)
		})
	}
}

// Tests PutCurrentValidator, DeleteCurrentValidator, GetCurrentValidator,
// GetValidatorWeightDiffs, GetValidatorPublicKeyDiffs
func TestStateAddRemoveValidator(t *testing.T) {
	require := require.New(t)

	state, _ := newInitializedState(require)

	var (
		numNodes   = 3
		supernetID = ids.GenerateTestID()
		startTime  = time.Now()
		endTime    = startTime.Add(24 * time.Hour)
		stakers    = make([]Staker, numNodes)
	)
	for i := 0; i < numNodes; i++ {
		stakers[i] = Staker{
			TxID:            ids.GenerateTestID(),
			NodeID:          ids.GenerateTestNodeID(),
			Weight:          uint64(i + 1),
			StartTime:       startTime.Add(time.Duration(i) * time.Second),
			EndTime:         endTime.Add(time.Duration(i) * time.Second),
			PotentialReward: uint64(i + 1),
		}
		if i%2 == 0 {
			stakers[i].SupernetID = supernetID
		} else {
			sk, err := bls.NewSecretKey()
			require.NoError(err)
			stakers[i].PublicKey = bls.PublicFromSecretKey(sk)
			stakers[i].SupernetID = constants.PrimaryNetworkID
		}
	}

	type diff struct {
		added                            []Staker
		removed                          []Staker
		expectedSupernetWeightDiff       map[ids.NodeID]*ValidatorWeightDiff
		expectedPrimaryNetworkWeightDiff map[ids.NodeID]*ValidatorWeightDiff
		expectedPublicKeyDiff            map[ids.NodeID]*bls.PublicKey
	}
	diffs := []diff{
		{
			// Add a supernet validator
			added:                            []Staker{stakers[0]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[0].NodeID: {
					Decrease: false,
					Amount:   stakers[0].Weight,
				},
			},
			// No diff because this is a supernet validator
			expectedPublicKeyDiff: map[ids.NodeID]*bls.PublicKey{},
		},
		{
			// Remove a supernet validator
			removed:                          []Staker{stakers[0]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[0].NodeID: {
					Decrease: true,
					Amount:   stakers[0].Weight,
				},
			},
			// No diff because this is a supernet validator
			expectedPublicKeyDiff: map[ids.NodeID]*bls.PublicKey{},
		},
		{ // Add a primary network validator
			added: []Staker{stakers[1]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[1].NodeID: {
					Decrease: false,
					Amount:   stakers[1].Weight,
				},
			},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{},
			expectedPublicKeyDiff:      map[ids.NodeID]*bls.PublicKey{},
		},
		{ // Remove a primary network validator
			removed: []Staker{stakers[1]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[1].NodeID: {
					Decrease: true,
					Amount:   stakers[1].Weight,
				},
			},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{},
			expectedPublicKeyDiff: map[ids.NodeID]*bls.PublicKey{
				stakers[1].NodeID: stakers[1].PublicKey,
			},
		},
		{
			// Add 2 supernet validators and a primary network validator
			added: []Staker{stakers[0], stakers[1], stakers[2]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[1].NodeID: {
					Decrease: false,
					Amount:   stakers[1].Weight,
				},
			},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[0].NodeID: {
					Decrease: false,
					Amount:   stakers[0].Weight,
				},
				stakers[2].NodeID: {
					Decrease: false,
					Amount:   stakers[2].Weight,
				},
			},
			expectedPublicKeyDiff: map[ids.NodeID]*bls.PublicKey{},
		},
		{
			// Remove 2 supernet validators and a primary network validator.
			removed: []Staker{stakers[0], stakers[1], stakers[2]},
			expectedPrimaryNetworkWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[1].NodeID: {
					Decrease: true,
					Amount:   stakers[1].Weight,
				},
			},
			expectedSupernetWeightDiff: map[ids.NodeID]*ValidatorWeightDiff{
				stakers[0].NodeID: {
					Decrease: true,
					Amount:   stakers[0].Weight,
				},
				stakers[2].NodeID: {
					Decrease: true,
					Amount:   stakers[2].Weight,
				},
			},
			expectedPublicKeyDiff: map[ids.NodeID]*bls.PublicKey{
				stakers[1].NodeID: stakers[1].PublicKey,
			},
		},
	}

	for i, diff := range diffs {
		for _, added := range diff.added {
			added := added
			state.PutCurrentValidator(&added)
		}
		for _, removed := range diff.removed {
			removed := removed
			state.DeleteCurrentValidator(&removed)
		}

		newHeight := uint64(i + 1)
		state.SetHeight(newHeight)

		require.NoError(state.Commit())

		for _, added := range diff.added {
			gotValidator, err := state.GetCurrentValidator(added.SupernetID, added.NodeID)
			require.NoError(err)
			require.Equal(added, *gotValidator)
		}

		for _, removed := range diff.removed {
			_, err := state.GetCurrentValidator(removed.SupernetID, removed.NodeID)
			require.ErrorIs(err, database.ErrNotFound)
		}

		// Assert that we get the expected weight diffs
		gotSupernetWeightDiffs, err := state.GetValidatorWeightDiffs(newHeight, supernetID)
		require.NoError(err)
		require.Equal(diff.expectedSupernetWeightDiff, gotSupernetWeightDiffs)

		gotWeightDiffs, err := state.GetValidatorWeightDiffs(newHeight, constants.PrimaryNetworkID)
		require.NoError(err)
		require.Equal(diff.expectedPrimaryNetworkWeightDiff, gotWeightDiffs)

		// Assert that we get the expected public key diff
		gotPublicKeyDiffs, err := state.GetValidatorPublicKeyDiffs(newHeight)
		require.NoError(err)
		require.Equal(diff.expectedPublicKeyDiff, gotPublicKeyDiffs)
	}
}
