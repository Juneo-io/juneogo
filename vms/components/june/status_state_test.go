// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package june

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/choices"
)

func TestStatusState(t *testing.T) {
	require := require.New(t)
	id0 := ids.GenerateTestID()

	db := memdb.New()
	s := NewStatusState(db)

	_, err := s.GetStatus(id0)
	require.Equal(database.ErrNotFound, err)

	_, err = s.GetStatus(id0)
	require.Equal(database.ErrNotFound, err)

	err = s.PutStatus(id0, choices.Accepted)
	require.NoError(err)

	status, err := s.GetStatus(id0)
	require.NoError(err)
	require.Equal(choices.Accepted, status)

	err = s.DeleteStatus(id0)
	require.NoError(err)

	_, err = s.GetStatus(id0)
	require.Equal(database.ErrNotFound, err)

	err = s.PutStatus(id0, choices.Accepted)
	require.NoError(err)

	s = NewStatusState(db)

	status, err = s.GetStatus(id0)
	require.NoError(err)
	require.Equal(choices.Accepted, status)
}
