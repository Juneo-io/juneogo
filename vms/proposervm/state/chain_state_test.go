// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/ids"
)

func testChainState(a *require.Assertions, cs ChainState) {
	lastAccepted := ids.GenerateTestID()

	_, err := cs.GetLastAccepted()
	a.Equal(database.ErrNotFound, err)

	err = cs.SetLastAccepted(lastAccepted)
	a.NoError(err)

	err = cs.SetLastAccepted(lastAccepted)
	a.NoError(err)

	fetchedLastAccepted, err := cs.GetLastAccepted()
	a.NoError(err)
	a.Equal(lastAccepted, fetchedLastAccepted)

	fetchedLastAccepted, err = cs.GetLastAccepted()
	a.NoError(err)
	a.Equal(lastAccepted, fetchedLastAccepted)

	err = cs.DeleteLastAccepted()
	a.NoError(err)

	_, err = cs.GetLastAccepted()
	a.Equal(database.ErrNotFound, err)
}

func TestChainState(t *testing.T) {
	a := require.New(t)

	db := memdb.New()
	cs := NewChainState(db)

	testChainState(a, cs)
}
