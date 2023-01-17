// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package uptime

import (
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
)

var _ State = (*TestState)(nil)

type uptime struct {
	upDuration  time.Duration
	lastUpdated time.Time
	startTime   time.Time
}

type TestState struct {
	dbReadError  error
	dbWriteError error
	nodes        map[ids.NodeID]map[ids.ID]*uptime
}

func NewTestState() *TestState {
	return &TestState{
		nodes: make(map[ids.NodeID]map[ids.ID]*uptime),
	}
}

func (s *TestState) AddNode(nodeID ids.NodeID, supernetID ids.ID, startTime time.Time) {
	supernetUptimes, ok := s.nodes[nodeID]
	if !ok {
		supernetUptimes = make(map[ids.ID]*uptime)
		s.nodes[nodeID] = supernetUptimes
	}
	st := time.Unix(startTime.Unix(), 0)
	supernetUptimes[supernetID] = &uptime{
		lastUpdated: st,
		startTime:   st,
	}
}

func (s *TestState) GetUptime(nodeID ids.NodeID, supernetID ids.ID) (time.Duration, time.Time, error) {
	up, exists := s.nodes[nodeID][supernetID]
	if !exists {
		return 0, time.Time{}, database.ErrNotFound
	}
	return up.upDuration, up.lastUpdated, s.dbReadError
}

func (s *TestState) SetUptime(nodeID ids.NodeID, supernetID ids.ID, upDuration time.Duration, lastUpdated time.Time) error {
	up, exists := s.nodes[nodeID][supernetID]
	if !exists {
		return database.ErrNotFound
	}
	up.upDuration = upDuration
	up.lastUpdated = time.Unix(lastUpdated.Unix(), 0)
	return s.dbWriteError
}

func (s *TestState) GetStartTime(nodeID ids.NodeID, supernetID ids.ID) (time.Time, error) {
	up, exists := s.nodes[nodeID][supernetID]
	if !exists {
		return time.Time{}, database.ErrNotFound
	}
	return up.startTime, s.dbReadError
}
