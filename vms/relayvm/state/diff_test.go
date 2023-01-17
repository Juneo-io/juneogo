// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/status"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

func TestDiffMissingState(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	versions := NewMockVersions(ctrl)

	parentID := ids.GenerateTestID()
	versions.EXPECT().GetState(parentID).Times(1).Return(nil, false)

	_, err := NewDiff(parentID, versions)
	require.ErrorIs(err, ErrMissingParentState)
}

func TestDiffCreation(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lastAcceptedID := ids.GenerateTestID()
	state, _ := newInitializedState(require)
	versions := NewMockVersions(ctrl)
	versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(state, true)

	d, err := NewDiff(lastAcceptedID, versions)
	require.NoError(err)
	assertChainsEqual(t, state, d)
}

func TestDiffCurrentSupply(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lastAcceptedID := ids.GenerateTestID()
	state, _ := newInitializedState(require)
	versions := NewMockVersions(ctrl)
	versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(state, true)

	d, err := NewDiff(lastAcceptedID, versions)
	require.NoError(err)

	initialCurrentSupply, err := d.GetCurrentSupply(constants.PrimaryNetworkID)
	require.NoError(err)

	newCurrentSupply := initialCurrentSupply + 1
	d.SetCurrentSupply(constants.PrimaryNetworkID, newCurrentSupply)

	returnedNewCurrentSupply, err := d.GetCurrentSupply(constants.PrimaryNetworkID)
	require.NoError(err)
	require.Equal(newCurrentSupply, returnedNewCurrentSupply)

	returnedBaseCurrentSupply, err := state.GetCurrentSupply(constants.PrimaryNetworkID)
	require.NoError(err)
	require.Equal(initialCurrentSupply, returnedBaseCurrentSupply)
}

func TestDiffCurrentValidator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lastAcceptedID := ids.GenerateTestID()
	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a current validator
	currentValidator := &Staker{
		TxID:       ids.GenerateTestID(),
		SupernetID: ids.GenerateTestID(),
		NodeID:     ids.GenerateTestNodeID(),
	}
	d.PutCurrentValidator(currentValidator)

	// Assert that we get the current validator back
	gotCurrentValidator, err := d.GetCurrentValidator(currentValidator.SupernetID, currentValidator.NodeID)
	require.NoError(err)
	require.Equal(currentValidator, gotCurrentValidator)

	// Delete the current validator
	d.DeleteCurrentValidator(currentValidator)

	// Make sure the deletion worked
	_, err = d.GetCurrentValidator(currentValidator.SupernetID, currentValidator.NodeID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestDiffPendingValidator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lastAcceptedID := ids.GenerateTestID()
	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a pending validator
	pendingValidator := &Staker{
		TxID:       ids.GenerateTestID(),
		SupernetID: ids.GenerateTestID(),
		NodeID:     ids.GenerateTestNodeID(),
	}
	d.PutPendingValidator(pendingValidator)

	// Assert that we get the pending validator back
	gotPendingValidator, err := d.GetPendingValidator(pendingValidator.SupernetID, pendingValidator.NodeID)
	require.NoError(err)
	require.Equal(pendingValidator, gotPendingValidator)

	// Delete the pending validator
	d.DeletePendingValidator(pendingValidator)

	// Make sure the deletion worked
	_, err = d.GetPendingValidator(pendingValidator.SupernetID, pendingValidator.NodeID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestDiffCurrentDelegator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	currentDelegator := &Staker{
		TxID:       ids.GenerateTestID(),
		SupernetID: ids.GenerateTestID(),
		NodeID:     ids.GenerateTestNodeID(),
	}

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a current delegator
	d.PutCurrentDelegator(currentDelegator)

	// Assert that we get the current delegator back
	// Mock iterator for [state] returns no delegators.
	stateCurrentDelegatorIter := NewMockStakerIterator(ctrl)
	stateCurrentDelegatorIter.EXPECT().Next().Return(false).Times(2)
	stateCurrentDelegatorIter.EXPECT().Release().Times(2)
	state.EXPECT().GetCurrentDelegatorIterator(
		currentDelegator.SupernetID,
		currentDelegator.NodeID,
	).Return(stateCurrentDelegatorIter, nil).Times(2)
	gotCurrentDelegatorIter, err := d.GetCurrentDelegatorIterator(currentDelegator.SupernetID, currentDelegator.NodeID)
	require.NoError(err)
	// The iterator should have the 1 delegator we put in [d]
	require.True(gotCurrentDelegatorIter.Next())
	require.Equal(gotCurrentDelegatorIter.Value(), currentDelegator)

	// Delete the current delegator
	d.DeleteCurrentDelegator(currentDelegator)

	// Make sure the deletion worked.
	// The iterator should have no elements.
	gotCurrentDelegatorIter, err = d.GetCurrentDelegatorIterator(currentDelegator.SupernetID, currentDelegator.NodeID)
	require.NoError(err)
	require.False(gotCurrentDelegatorIter.Next())
}

func TestDiffPendingDelegator(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pendingDelegator := &Staker{
		TxID:       ids.GenerateTestID(),
		SupernetID: ids.GenerateTestID(),
		NodeID:     ids.GenerateTestNodeID(),
	}

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a pending delegator
	d.PutPendingDelegator(pendingDelegator)

	// Assert that we get the pending delegator back
	// Mock iterator for [state] returns no delegators.
	statePendingDelegatorIter := NewMockStakerIterator(ctrl)
	statePendingDelegatorIter.EXPECT().Next().Return(false).Times(2)
	statePendingDelegatorIter.EXPECT().Release().Times(2)
	state.EXPECT().GetPendingDelegatorIterator(
		pendingDelegator.SupernetID,
		pendingDelegator.NodeID,
	).Return(statePendingDelegatorIter, nil).Times(2)
	gotPendingDelegatorIter, err := d.GetPendingDelegatorIterator(pendingDelegator.SupernetID, pendingDelegator.NodeID)
	require.NoError(err)
	// The iterator should have the 1 delegator we put in [d]
	require.True(gotPendingDelegatorIter.Next())
	require.Equal(gotPendingDelegatorIter.Value(), pendingDelegator)

	// Delete the pending delegator
	d.DeletePendingDelegator(pendingDelegator)

	// Make sure the deletion worked.
	// The iterator should have no elements.
	gotPendingDelegatorIter, err = d.GetPendingDelegatorIterator(pendingDelegator.SupernetID, pendingDelegator.NodeID)
	require.NoError(err)
	require.False(gotPendingDelegatorIter.Next())
}

func TestDiffSupernet(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a supernet
	createSupernetTx := &txs.Tx{}
	d.AddSupernet(createSupernetTx)

	// Assert that we get the supernet back
	// [state] returns 1 supernet.
	parentStateCreateSupernetTx := &txs.Tx{}
	state.EXPECT().GetSupernets().Return([]*txs.Tx{parentStateCreateSupernetTx}, nil).Times(1)
	gotSupernets, err := d.GetSupernets()
	require.NoError(err)
	require.Len(gotSupernets, 2)
	require.Equal(gotSupernets[0], parentStateCreateSupernetTx)
	require.Equal(gotSupernets[1], createSupernetTx)
}

func TestDiffChain(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a chain
	supernetID := ids.GenerateTestID()
	createChainTx := &txs.Tx{
		Unsigned: &txs.CreateChainTx{
			SupernetID: supernetID,
		},
	}
	d.AddChain(createChainTx)

	// Assert that we get the chain back
	// [state] returns 1 chain.
	parentStateCreateChainTx := &txs.Tx{
		Unsigned: &txs.CreateChainTx{
			SupernetID: supernetID, // note this is the same supernet as [createChainTx]
		},
	}
	state.EXPECT().GetChains(supernetID).Return([]*txs.Tx{parentStateCreateChainTx}, nil).Times(1)
	gotChains, err := d.GetChains(supernetID)
	require.NoError(err)
	require.Len(gotChains, 2)
	require.Equal(gotChains[0], parentStateCreateChainTx)
	require.Equal(gotChains[1], createChainTx)
}

func TestDiffTx(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a tx
	supernetID := ids.GenerateTestID()
	tx := &txs.Tx{
		Unsigned: &txs.CreateChainTx{
			SupernetID: supernetID,
		},
	}
	tx.SetBytes(utils.RandomBytes(16), utils.RandomBytes(16))
	d.AddTx(tx, status.Committed, ids.Empty)

	{
		// Assert that we get the tx back
		gotTx, gotStatus, err := d.GetTx(tx.ID())
		require.NoError(err)
		require.Equal(status.Committed, gotStatus)
		require.Equal(tx, gotTx)
	}

	{
		// Assert that we can get a tx from the parent state
		// [state] returns 1 tx.
		parentTx := &txs.Tx{
			Unsigned: &txs.CreateChainTx{
				SupernetID: supernetID,
			},
		}
		parentTx.SetBytes(utils.RandomBytes(16), utils.RandomBytes(16))
		state.EXPECT().GetTx(parentTx.ID()).Return(parentTx, status.Committed, nil).Times(1)
		gotParentTx, gotStatus, err := d.GetTx(parentTx.ID())
		require.NoError(err)
		require.Equal(status.Committed, gotStatus)
		require.Equal(parentTx, gotParentTx)
	}
}

func TestDiffRewardUTXO(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a reward UTXO
	txID := ids.GenerateTestID()
	rewardUTXO := &june.UTXO{
		UTXOID: june.UTXOID{TxID: txID},
	}
	d.AddRewardUTXO(txID, rewardUTXO)

	{
		// Assert that we get the UTXO back
		gotRewardUTXOs, err := d.GetRewardUTXOs(txID)
		require.NoError(err)
		require.Len(gotRewardUTXOs, 1)
		require.Equal(rewardUTXO, gotRewardUTXOs[0])
	}

	{
		// Assert that we can get a UTXO from the parent state
		// [state] returns 1 UTXO.
		txID2 := ids.GenerateTestID()
		parentRewardUTXO := &june.UTXO{
			UTXOID: june.UTXOID{TxID: txID2},
		}
		state.EXPECT().GetRewardUTXOs(txID2).Return([]*june.UTXO{parentRewardUTXO}, nil).Times(1)
		gotParentRewardUTXOs, err := d.GetRewardUTXOs(txID2)
		require.NoError(err)
		require.Len(gotParentRewardUTXOs, 1)
		require.Equal(parentRewardUTXO, gotParentRewardUTXOs[0])
	}
}

func TestDiffUTXO(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := NewMockState(ctrl)
	// Called in NewDiff
	state.EXPECT().GetTimestamp().Return(time.Now()).Times(1)

	states := NewMockVersions(ctrl)
	lastAcceptedID := ids.GenerateTestID()
	states.EXPECT().GetState(lastAcceptedID).Return(state, true).AnyTimes()

	d, err := NewDiff(lastAcceptedID, states)
	require.NoError(err)

	// Put a UTXO
	utxo := &june.UTXO{
		UTXOID: june.UTXOID{TxID: ids.GenerateTestID()},
	}
	d.AddUTXO(utxo)

	{
		// Assert that we get the UTXO back
		gotUTXO, err := d.GetUTXO(utxo.InputID())
		require.NoError(err)
		require.Equal(utxo, gotUTXO)
	}

	{
		// Assert that we can get a UTXO from the parent state
		// [state] returns 1 UTXO.
		parentUTXO := &june.UTXO{
			UTXOID: june.UTXOID{TxID: ids.GenerateTestID()},
		}
		state.EXPECT().GetUTXO(parentUTXO.InputID()).Return(parentUTXO, nil).Times(1)
		gotParentUTXO, err := d.GetUTXO(parentUTXO.InputID())
		require.NoError(err)
		require.Equal(parentUTXO, gotParentUTXO)
	}

	{
		// Delete the UTXO
		d.DeleteUTXO(utxo.InputID())

		// Make sure it's gone
		_, err = d.GetUTXO(utxo.InputID())
		require.Error(err)
	}
}

func assertChainsEqual(t *testing.T, expected, actual Chain) {
	t.Helper()

	expectedCurrentStakerIterator, expectedErr := expected.GetCurrentStakerIterator()
	actualCurrentStakerIterator, actualErr := actual.GetCurrentStakerIterator()
	require.Equal(t, expectedErr, actualErr)
	if expectedErr == nil {
		assertIteratorsEqual(t, expectedCurrentStakerIterator, actualCurrentStakerIterator)
	}

	expectedPendingStakerIterator, expectedErr := expected.GetPendingStakerIterator()
	actualPendingStakerIterator, actualErr := actual.GetPendingStakerIterator()
	require.Equal(t, expectedErr, actualErr)
	if expectedErr == nil {
		assertIteratorsEqual(t, expectedPendingStakerIterator, actualPendingStakerIterator)
	}

	require.Equal(t, expected.GetTimestamp(), actual.GetTimestamp())

	expectedCurrentSupply, err := expected.GetCurrentSupply(constants.PrimaryNetworkID)
	require.NoError(t, err)

	actualCurrentSupply, err := actual.GetCurrentSupply(constants.PrimaryNetworkID)
	require.NoError(t, err)

	require.Equal(t, expectedCurrentSupply, actualCurrentSupply)

	expectedSupernets, expectedErr := expected.GetSupernets()
	actualSupernets, actualErr := actual.GetSupernets()
	require.Equal(t, expectedErr, actualErr)
	if expectedErr == nil {
		require.Equal(t, expectedSupernets, actualSupernets)

		for _, supernet := range expectedSupernets {
			supernetID := supernet.ID()

			expectedChains, expectedErr := expected.GetChains(supernetID)
			actualChains, actualErr := actual.GetChains(supernetID)
			require.Equal(t, expectedErr, actualErr)
			if expectedErr == nil {
				require.Equal(t, expectedChains, actualChains)
			}
		}
	}
}
