// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chains

import (
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/engine/common"
)

// Registrant can register the existence of a chain
type Registrant interface {
	// Called when a chain is created
	// This function is called before the chain starts processing messages
	// [vm] should be a vertex.DAGVM or block.ChainVM
	RegisterChain(chainName string, ctx *snow.ConsensusContext, vm common.VM)
}
