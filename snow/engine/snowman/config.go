// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowman

import (
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/consensus/snowball"
	"github.com/Juneo-io/juneogo/snow/consensus/snowman"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/snow/engine/snowman/block"
	"github.com/Juneo-io/juneogo/snow/validators"
)

// Config wraps all the parameters needed for a snowman engine
type Config struct {
	common.AllGetsServer

	Ctx         *snow.ConsensusContext
	VM          block.ChainVM
	Sender      common.Sender
	Validators  validators.Manager
	Params      snowball.Parameters
	Consensus   snowman.Consensus
	PartialSync bool
}
