// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chains

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
)

func TestSupernet(t *testing.T) {
	require := require.New(t)

	chainID0 := ids.GenerateTestID()
	chainID1 := ids.GenerateTestID()
	chainID2 := ids.GenerateTestID()

	s := newSupernet()
	s.addChain(chainID0)
	require.False(s.IsBootstrapped(), "A supernet with one chain in bootstrapping shouldn't be considered bootstrapped")

	s.Bootstrapped(chainID0)
	require.True(s.IsBootstrapped(), "A supernet with only bootstrapped chains should be considered bootstrapped")

	s.addChain(chainID1)
	require.False(s.IsBootstrapped(), "A supernet with one chain in bootstrapping shouldn't be considered bootstrapped")

	s.addChain(chainID2)
	require.False(s.IsBootstrapped(), "A supernet with one chain in bootstrapping shouldn't be considered bootstrapped")

	s.Bootstrapped(chainID1)
	require.False(s.IsBootstrapped(), "A supernet with one chain in bootstrapping shouldn't be considered bootstrapped")

	s.Bootstrapped(chainID2)
	require.True(s.IsBootstrapped(), "A supernet with only bootstrapped chains should be considered bootstrapped")
}
