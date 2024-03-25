// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package proposer

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
)

var _ utils.Sortable[validatorData] = validatorData{}

type validatorData struct {
	id     ids.NodeID
	weight uint64
}

func (d validatorData) Compare(other validatorData) int {
	return d.id.Compare(other.id)
}
