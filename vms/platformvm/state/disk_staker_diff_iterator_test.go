// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thepudds/fzgen/fuzzer"

	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/ids"
)

func FuzzMarshalDiffKey(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		require := require.New(t)

		var (
			supernetID ids.ID
			height   uint64
			nodeID   ids.NodeID
		)
		fz := fuzzer.NewFuzzer(data)
		fz.Fill(&supernetID, &height, &nodeID)

		key := marshalDiffKey(supernetID, height, nodeID)
		parsedSupernetID, parsedHeight, parsedNodeID, err := unmarshalDiffKey(key)
		require.NoError(err)
		require.Equal(supernetID, parsedSupernetID)
		require.Equal(height, parsedHeight)
		require.Equal(nodeID, parsedNodeID)
	})
}

func FuzzUnmarshalDiffKey(f *testing.F) {
	f.Fuzz(func(t *testing.T, key []byte) {
		require := require.New(t)

		supernetID, height, nodeID, err := unmarshalDiffKey(key)
		if err != nil {
			require.ErrorIs(err, errUnexpectedDiffKeyLength)
			return
		}

		formattedKey := marshalDiffKey(supernetID, height, nodeID)
		require.Equal(key, formattedKey)
	})
}

func TestDiffIteration(t *testing.T) {
	require := require.New(t)

	db := memdb.New()

	supernetID0 := ids.GenerateTestID()
	supernetID1 := ids.GenerateTestID()

	nodeID0 := ids.BuildTestNodeID([]byte{0x00})
	nodeID1 := ids.BuildTestNodeID([]byte{0x01})

	supernetID0Height0NodeID0 := marshalDiffKey(supernetID0, 0, nodeID0)
	supernetID0Height1NodeID0 := marshalDiffKey(supernetID0, 1, nodeID0)
	supernetID0Height1NodeID1 := marshalDiffKey(supernetID0, 1, nodeID1)

	supernetID1Height0NodeID0 := marshalDiffKey(supernetID1, 0, nodeID0)
	supernetID1Height1NodeID0 := marshalDiffKey(supernetID1, 1, nodeID0)
	supernetID1Height1NodeID1 := marshalDiffKey(supernetID1, 1, nodeID1)

	require.NoError(db.Put(supernetID0Height0NodeID0, nil))
	require.NoError(db.Put(supernetID0Height1NodeID0, nil))
	require.NoError(db.Put(supernetID0Height1NodeID1, nil))
	require.NoError(db.Put(supernetID1Height0NodeID0, nil))
	require.NoError(db.Put(supernetID1Height1NodeID0, nil))
	require.NoError(db.Put(supernetID1Height1NodeID1, nil))

	{
		it := db.NewIteratorWithStartAndPrefix(marshalStartDiffKey(supernetID0, 0), supernetID0[:])
		defer it.Release()

		expectedKeys := [][]byte{
			supernetID0Height0NodeID0,
		}
		for _, expectedKey := range expectedKeys {
			require.True(it.Next())
			require.Equal(expectedKey, it.Key())
		}
		require.False(it.Next())
		require.NoError(it.Error())
	}

	{
		it := db.NewIteratorWithStartAndPrefix(marshalStartDiffKey(supernetID0, 1), supernetID0[:])
		defer it.Release()

		expectedKeys := [][]byte{
			supernetID0Height1NodeID0,
			supernetID0Height1NodeID1,
			supernetID0Height0NodeID0,
		}
		for _, expectedKey := range expectedKeys {
			require.True(it.Next())
			require.Equal(expectedKey, it.Key())
		}
		require.False(it.Next())
		require.NoError(it.Error())
	}
}
