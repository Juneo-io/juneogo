// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bootstrap

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/engine/avalanche/vertex"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/snow/engine/common/queue"
	"github.com/Juneo-io/juneogo/snow/engine/common/tracker"
	"github.com/Juneo-io/juneogo/snow/validators"
)

type Config struct {
	common.AllGetsServer

	Ctx     *snow.ConsensusContext
	Beacons validators.Manager

	StartupTracker tracker.Startup
	Sender         common.Sender

	// This node will only consider the first [AncestorsMaxContainersReceived]
	// containers in an ancestors message it receives.
	AncestorsMaxContainersReceived int

	// VtxBlocked tracks operations that are blocked on vertices
	VtxBlocked *queue.JobsWithMissing
	// TxBlocked tracks operations that are blocked on transactions
	TxBlocked *queue.Jobs

	Manager vertex.Manager
	VM      vertex.LinearizableVM

	// If StopVertexID is empty, the engine will generate the stop vertex based
	// on the current state.
	StopVertexID ids.ID
}
