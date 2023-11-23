// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
)

func TestOverriddenManager(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	nodeID1 := ids.GenerateTestNodeID()
	supernetID0 := ids.GenerateTestID()
	supernetID1 := ids.GenerateTestID()

	m := validators.NewManager()
	require.NoError(m.AddStaker(supernetID0, nodeID0, nil, ids.Empty, 1))
	require.NoError(m.AddStaker(supernetID1, nodeID1, nil, ids.Empty, 1))

	om := newOverriddenManager(supernetID0, m)
	_, ok := om.GetValidator(supernetID0, nodeID0)
	require.True(ok)
	_, ok = om.GetValidator(supernetID0, nodeID1)
	require.False(ok)
	_, ok = om.GetValidator(supernetID1, nodeID0)
	require.True(ok)
	_, ok = om.GetValidator(supernetID1, nodeID1)
	require.False(ok)

	require.NoError(om.RemoveWeight(supernetID1, nodeID0, 1))
	_, ok = om.GetValidator(supernetID0, nodeID0)
	require.False(ok)
	_, ok = om.GetValidator(supernetID0, nodeID1)
	require.False(ok)
	_, ok = om.GetValidator(supernetID1, nodeID0)
	require.False(ok)
	_, ok = om.GetValidator(supernetID1, nodeID1)
	require.False(ok)
}

func TestOverriddenString(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.EmptyNodeID
	nodeID1, err := ids.NodeIDFromString("NodeID-QLbz7JHiBTspS962RLKV8GndWFwdYhk6V")
	require.NoError(err)

	supernetID0, err := ids.FromString("TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES")
	require.NoError(err)
	supernetID1, err := ids.FromString("2mcwQKiD8VEspmMJpL1dc7okQQ5dDVAWeCBZ7FWBFAbxpv3t7w")
	require.NoError(err)

	m := validators.NewManager()
	require.NoError(m.AddStaker(supernetID0, nodeID0, nil, ids.Empty, 1))
	require.NoError(m.AddStaker(supernetID0, nodeID1, nil, ids.Empty, math.MaxInt64-1))
	require.NoError(m.AddStaker(supernetID1, nodeID1, nil, ids.Empty, 1))

	om := newOverriddenManager(supernetID0, m)
	expected := "Overridden Validator Manager (SupernetID = TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES): Validator Manager: (Size = 2)\n" +
		"    Supernet[TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES]: Validator Set: (Size = 2, Weight = 9223372036854775807)\n" +
		"        Validator[0]: NodeID-111111111111111111116DBWJs, 1\n" +
		"        Validator[1]: NodeID-QLbz7JHiBTspS962RLKV8GndWFwdYhk6V, 9223372036854775806\n" +
		"    Supernet[2mcwQKiD8VEspmMJpL1dc7okQQ5dDVAWeCBZ7FWBFAbxpv3t7w]: Validator Set: (Size = 1, Weight = 1)\n" +
		"        Validator[0]: NodeID-QLbz7JHiBTspS962RLKV8GndWFwdYhk6V, 1"
	result := om.String()
	require.Equal(expected, result)
}
