// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package handler

import (
	"errors"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/set"
)

var errDuplicatedID = errors.New("inbound message contains duplicated ID")

func getIDs(idsBytes [][]byte) ([]ids.ID, error) {
	res := make([]ids.ID, len(idsBytes))
	idSet := set.NewSet[ids.ID](len(idsBytes))
	for i, bytes := range idsBytes {
		id, err := ids.ToID(bytes)
		if err != nil {
			return nil, err
		}
		if idSet.Contains(id) {
			return nil, errDuplicatedID
		}
		res[i] = id
		idSet.Add(id)
	}
	return res, nil
}
