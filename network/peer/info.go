// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/set"
)

type Info struct {
	IP                    string                 `json:"ip"`
	PublicIP              string                 `json:"publicIP,omitempty"`
	ID                    ids.NodeID             `json:"nodeID"`
	Version               string                 `json:"version"`
	LastSent              time.Time              `json:"lastSent"`
	LastReceived          time.Time              `json:"lastReceived"`
	ObservedUptime        json.Uint32            `json:"observedUptime"`
	ObservedSupernetUptimes map[ids.ID]json.Uint32 `json:"observedSupernetUptimes"`
	TrackedSupernets        set.Set[ids.ID]        `json:"trackedSupernets"`
	SupportedACPs         set.Set[uint32]        `json:"supportedACPs"`
	ObjectedACPs          set.Set[uint32]        `json:"objectedACPs"`
}
