// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/wrappers"
)

func TestValidatorUptimes(t *testing.T) {
	require := require.New(t)
	state := newValidatorState()

	// get non-existent uptime
	nodeID := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	_, _, err := state.GetUptime(nodeID, supernetID)
	require.ErrorIs(err, database.ErrNotFound)

	// set non-existent uptime
	err = state.SetUptime(nodeID, supernetID, 1, time.Now())
	require.ErrorIs(err, database.ErrNotFound)

	testMetadata := &validatorMetadata{
		UpDuration:  time.Hour,
		lastUpdated: time.Now(),
	}
	// load uptime
	state.LoadValidatorMetadata(nodeID, supernetID, testMetadata)

	// get uptime
	upDuration, lastUpdated, err := state.GetUptime(nodeID, supernetID)
	require.NoError(err)
	require.Equal(testMetadata.UpDuration, upDuration)
	require.Equal(testMetadata.lastUpdated, lastUpdated)

	// set uptime
	newUpDuration := testMetadata.UpDuration + 1
	newLastUpdated := testMetadata.lastUpdated.Add(time.Hour)
	require.NoError(state.SetUptime(nodeID, supernetID, newUpDuration, newLastUpdated))

	// get new uptime
	upDuration, lastUpdated, err = state.GetUptime(nodeID, supernetID)
	require.NoError(err)
	require.Equal(newUpDuration, upDuration)
	require.Equal(newLastUpdated, lastUpdated)

	// load uptime changes uptimes
	newTestMetadata := &validatorMetadata{
		UpDuration:  testMetadata.UpDuration + time.Hour,
		lastUpdated: testMetadata.lastUpdated.Add(time.Hour),
	}
	state.LoadValidatorMetadata(nodeID, supernetID, newTestMetadata)

	// get new uptime
	upDuration, lastUpdated, err = state.GetUptime(nodeID, supernetID)
	require.NoError(err)
	require.Equal(newTestMetadata.UpDuration, upDuration)
	require.Equal(newTestMetadata.lastUpdated, lastUpdated)

	// delete uptime
	state.DeleteValidatorMetadata(nodeID, supernetID)

	// get deleted uptime
	_, _, err = state.GetUptime(nodeID, supernetID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestWriteValidatorMetadata(t *testing.T) {
	require := require.New(t)
	state := newValidatorState()

	primaryDB := memdb.New()
	supernetDB := memdb.New()
	// write empty uptimes
	require.NoError(state.WriteValidatorMetadata(primaryDB, supernetDB))

	// load uptime
	nodeID := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	testUptimeReward := &validatorMetadata{
		UpDuration:      time.Hour,
		lastUpdated:     time.Now(),
		PotentialReward: 100,
		txID:            ids.GenerateTestID(),
	}
	state.LoadValidatorMetadata(nodeID, supernetID, testUptimeReward)

	// write state, should not reflect to DB yet
	require.NoError(state.WriteValidatorMetadata(primaryDB, supernetDB))
	require.False(primaryDB.Has(testUptimeReward.txID[:]))
	require.False(supernetDB.Has(testUptimeReward.txID[:]))

	// get uptime should still return the loaded value
	upDuration, lastUpdated, err := state.GetUptime(nodeID, supernetID)
	require.NoError(err)
	require.Equal(testUptimeReward.UpDuration, upDuration)
	require.Equal(testUptimeReward.lastUpdated, lastUpdated)

	// update uptimes
	newUpDuration := testUptimeReward.UpDuration + 1
	newLastUpdated := testUptimeReward.lastUpdated.Add(time.Hour)
	require.NoError(state.SetUptime(nodeID, supernetID, newUpDuration, newLastUpdated))

	// write uptimes, should reflect to supernet DB
	require.NoError(state.WriteValidatorMetadata(primaryDB, supernetDB))
	require.False(primaryDB.Has(testUptimeReward.txID[:]))
	require.True(supernetDB.Has(testUptimeReward.txID[:]))
}

func TestValidatorDelegateeRewards(t *testing.T) {
	require := require.New(t)
	state := newValidatorState()

	// get non-existent delegatee reward
	nodeID := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	_, err := state.GetDelegateeReward(supernetID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// set non-existent delegatee reward
	err = state.SetDelegateeReward(supernetID, nodeID, 100000)
	require.ErrorIs(err, database.ErrNotFound)

	testMetadata := &validatorMetadata{
		PotentialDelegateeReward: 100000,
	}
	// load delegatee reward
	state.LoadValidatorMetadata(nodeID, supernetID, testMetadata)

	// get delegatee reward
	delegateeReward, err := state.GetDelegateeReward(supernetID, nodeID)
	require.NoError(err)
	require.Equal(testMetadata.PotentialDelegateeReward, delegateeReward)

	// set delegatee reward
	newDelegateeReward := testMetadata.PotentialDelegateeReward + 100000
	require.NoError(state.SetDelegateeReward(supernetID, nodeID, newDelegateeReward))

	// get new delegatee reward
	delegateeReward, err = state.GetDelegateeReward(supernetID, nodeID)
	require.NoError(err)
	require.Equal(newDelegateeReward, delegateeReward)

	// load delegatee reward changes
	newTestMetadata := &validatorMetadata{
		PotentialDelegateeReward: testMetadata.PotentialDelegateeReward + 100000,
	}
	state.LoadValidatorMetadata(nodeID, supernetID, newTestMetadata)

	// get new delegatee reward
	delegateeReward, err = state.GetDelegateeReward(supernetID, nodeID)
	require.NoError(err)
	require.Equal(newTestMetadata.PotentialDelegateeReward, delegateeReward)

	// delete delegatee reward
	state.DeleteValidatorMetadata(nodeID, supernetID)

	// get deleted delegatee reward
	_, _, err = state.GetUptime(nodeID, supernetID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestParseValidatorMetadata(t *testing.T) {
	type test struct {
		name        string
		bytes       []byte
		expected    *validatorMetadata
		expectedErr error
	}
	tests := []test{
		{
			name:  "nil",
			bytes: nil,
			expected: &validatorMetadata{
				lastUpdated: time.Unix(0, 0),
			},
			expectedErr: nil,
		},
		{
			name:  "nil",
			bytes: []byte{},
			expected: &validatorMetadata{
				lastUpdated: time.Unix(0, 0),
			},
			expectedErr: nil,
		},
		{
			name: "potential reward only",
			bytes: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x86, 0xA0,
			},
			expected: &validatorMetadata{
				PotentialReward: 100000,
				lastUpdated:     time.Unix(0, 0),
			},
			expectedErr: nil,
		},
		{
			name: "uptime + potential reward",
			bytes: []byte{
				// codec version
				0x00, 0x00,
				// up duration
				0x00, 0x00, 0x00, 0x00, 0x00, 0x5B, 0x8D, 0x80,
				// last updated
				0x00, 0x00, 0x00, 0x00, 0x00, 0x0D, 0xBB, 0xA0,
				// potential reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x86, 0xA0,
			},
			expected: &validatorMetadata{
				UpDuration:      6000000,
				LastUpdated:     900000,
				PotentialReward: 100000,
				lastUpdated:     time.Unix(900000, 0),
			},
			expectedErr: nil,
		},
		{
			name: "uptime + potential reward + potential delegatee reward",
			bytes: []byte{
				// codec version
				0x00, 0x00,
				// up duration
				0x00, 0x00, 0x00, 0x00, 0x00, 0x5B, 0x8D, 0x80,
				// last updated
				0x00, 0x00, 0x00, 0x00, 0x00, 0x0D, 0xBB, 0xA0,
				// potential reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x86, 0xA0,
				// potential delegatee reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4E, 0x20,
			},
			expected: &validatorMetadata{
				UpDuration:               6000000,
				LastUpdated:              900000,
				PotentialReward:          100000,
				PotentialDelegateeReward: 20000,
				lastUpdated:              time.Unix(900000, 0),
			},
			expectedErr: nil,
		},
		{
			name: "invalid codec version",
			bytes: []byte{
				// codec version
				0x00, 0x01,
				// up duration
				0x00, 0x00, 0x00, 0x00, 0x00, 0x5B, 0x8D, 0x80,
				// last updated
				0x00, 0x00, 0x00, 0x00, 0x00, 0x0D, 0xBB, 0xA0,
				// potential reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x86, 0xA0,
				// potential delegatee reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4E, 0x20,
			},
			expected:    nil,
			expectedErr: codec.ErrUnknownVersion,
		},
		{
			name: "short byte len",
			bytes: []byte{
				// codec version
				0x00, 0x00,
				// up duration
				0x00, 0x00, 0x00, 0x00, 0x00, 0x5B, 0x8D, 0x80,
				// last updated
				0x00, 0x00, 0x00, 0x00, 0x00, 0x0D, 0xBB, 0xA0,
				// potential reward
				0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x86, 0xA0,
				// potential delegatee reward
				0x00, 0x00, 0x00, 0x00, 0x4E, 0x20,
			},
			expected:    nil,
			expectedErr: wrappers.ErrInsufficientLength,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			var metadata validatorMetadata
			err := parseValidatorMetadata(tt.bytes, &metadata)
			require.ErrorIs(err, tt.expectedErr)
			if tt.expectedErr != nil {
				return
			}
			require.Equal(tt.expected, &metadata)
		})
	}
}
