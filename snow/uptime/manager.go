// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package uptime

import (
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
)

var _ Manager = (*manager)(nil)

type Manager interface {
	Tracker
	Calculator
}

type Tracker interface {
	StartTracking(nodeIDs []ids.NodeID, supernetID ids.ID) error
	StopTracking(nodeIDs []ids.NodeID, supernetID ids.ID) error

	Connect(nodeID ids.NodeID, supernetID ids.ID) error
	IsConnected(nodeID ids.NodeID, supernetID ids.ID) bool
	Disconnect(nodeID ids.NodeID) error
}

type Calculator interface {
	CalculateUptime(nodeID ids.NodeID, supernetID ids.ID) (time.Duration, time.Time, error)
	CalculateUptimePercent(nodeID ids.NodeID, supernetID ids.ID) (float64, error)
	// CalculateUptimePercentFrom expects [startTime] to be truncated (floored) to the nearest second
	CalculateUptimePercentFrom(nodeID ids.NodeID, supernetID ids.ID, startTime time.Time) (float64, error)
}

type manager struct {
	// Used to get time. Useful for faking time during tests.
	clock *mockable.Clock

	state          State
	connections    map[ids.NodeID]map[ids.ID]time.Time // nodeID -> supernetID -> time
	trackedSupernets set.Set[ids.ID]
}

func NewManager(state State, clk *mockable.Clock) Manager {
	return &manager{
		clock:       clk,
		state:       state,
		connections: make(map[ids.NodeID]map[ids.ID]time.Time),
	}
}

func (m *manager) StartTracking(nodeIDs []ids.NodeID, supernetID ids.ID) error {
	now := m.clock.UnixTime()
	for _, nodeID := range nodeIDs {
		upDuration, lastUpdated, err := m.state.GetUptime(nodeID, supernetID)
		if err != nil {
			return err
		}

		// If we are in a weird reality where time has moved backwards, then we
		// shouldn't modify the validator's uptime.
		if now.Before(lastUpdated) {
			continue
		}

		durationOffline := now.Sub(lastUpdated)
		newUpDuration := upDuration + durationOffline
		if err := m.state.SetUptime(nodeID, supernetID, newUpDuration, now); err != nil {
			return err
		}
	}
	m.trackedSupernets.Add(supernetID)
	return nil
}

func (m *manager) StopTracking(nodeIDs []ids.NodeID, supernetID ids.ID) error {
	now := m.clock.UnixTime()
	for _, nodeID := range nodeIDs {
		connectedSupernets := m.connections[nodeID]
		// If the node is already connected to this supernet, then we can just
		// update the uptime in the state and remove the connection
		if _, isConnected := connectedSupernets[supernetID]; isConnected {
			if err := m.updateSupernetUptime(nodeID, supernetID); err != nil {
				delete(connectedSupernets, supernetID)
				return err
			}
			delete(connectedSupernets, supernetID)
			continue
		}

		// if the node is not connected to this supernet, then we need to update
		// the uptime in the state from the last time the node was connected to
		// this supernet to now.
		upDuration, lastUpdated, err := m.state.GetUptime(nodeID, supernetID)
		if err != nil {
			return err
		}

		// If we are in a weird reality where time has moved backwards, then we
		// shouldn't modify the validator's uptime.
		if now.Before(lastUpdated) {
			continue
		}

		if err := m.state.SetUptime(nodeID, supernetID, upDuration, now); err != nil {
			return err
		}
	}
	return nil
}

func (m *manager) Connect(nodeID ids.NodeID, supernetID ids.ID) error {
	supernetConnections, ok := m.connections[nodeID]
	if !ok {
		supernetConnections = make(map[ids.ID]time.Time)
		m.connections[nodeID] = supernetConnections
	}
	supernetConnections[supernetID] = m.clock.UnixTime()
	return nil
}

func (m *manager) IsConnected(nodeID ids.NodeID, supernetID ids.ID) bool {
	_, connected := m.connections[nodeID][supernetID]
	return connected
}

func (m *manager) Disconnect(nodeID ids.NodeID) error {
	// Update every supernet that this node was connected to
	for supernetID := range m.connections[nodeID] {
		if err := m.updateSupernetUptime(nodeID, supernetID); err != nil {
			return err
		}
	}
	delete(m.connections, nodeID)
	return nil
}

func (m *manager) CalculateUptime(nodeID ids.NodeID, supernetID ids.ID) (time.Duration, time.Time, error) {
	upDuration, lastUpdated, err := m.state.GetUptime(nodeID, supernetID)
	if err != nil {
		return 0, time.Time{}, err
	}

	now := m.clock.UnixTime()
	// If we are in a weird reality where time has gone backwards, make sure
	// that we don't double count or delete any uptime.
	if now.Before(lastUpdated) {
		return upDuration, lastUpdated, nil
	}

	if !m.trackedSupernets.Contains(supernetID) {
		durationOffline := now.Sub(lastUpdated)
		newUpDuration := upDuration + durationOffline
		return newUpDuration, now, nil
	}

	timeConnected, isConnected := m.connections[nodeID][supernetID]
	if !isConnected {
		return upDuration, now, nil
	}

	// The time the peer connected needs to be adjusted to ensure no time period
	// is double counted.
	if timeConnected.Before(lastUpdated) {
		timeConnected = lastUpdated
	}

	// If we are in a weird reality where time has gone backwards, make sure
	// that we don't double count or delete any uptime.
	if now.Before(timeConnected) {
		return upDuration, now, nil
	}

	// Increase the uptimes by the amount of time this node has been running
	// since the last time it's uptime was written to disk.
	durationConnected := now.Sub(timeConnected)
	newUpDuration := upDuration + durationConnected
	return newUpDuration, now, nil
}

func (m *manager) CalculateUptimePercent(nodeID ids.NodeID, supernetID ids.ID) (float64, error) {
	startTime, err := m.state.GetStartTime(nodeID, supernetID)
	if err != nil {
		return 0, err
	}
	return m.CalculateUptimePercentFrom(nodeID, supernetID, startTime)
}

func (m *manager) CalculateUptimePercentFrom(nodeID ids.NodeID, supernetID ids.ID, startTime time.Time) (float64, error) {
	upDuration, now, err := m.CalculateUptime(nodeID, supernetID)
	if err != nil {
		return 0, err
	}
	bestPossibleUpDuration := now.Sub(startTime)
	if bestPossibleUpDuration == 0 {
		return 1, nil
	}
	uptime := float64(upDuration) / float64(bestPossibleUpDuration)
	return uptime, nil
}

// updateSupernetUptime updates the supernet uptime of the node on the state by the amount
// of time that the node has been connected to the supernet.
func (m *manager) updateSupernetUptime(nodeID ids.NodeID, supernetID ids.ID) error {
	// we're not tracking this supernet, skip updating it.
	if !m.trackedSupernets.Contains(supernetID) {
		return nil
	}

	newDuration, newLastUpdated, err := m.CalculateUptime(nodeID, supernetID)
	if err == database.ErrNotFound {
		// If a non-validator disconnects, we don't care
		return nil
	}
	if err != nil {
		return err
	}

	return m.state.SetUptime(nodeID, supernetID, newDuration, newLastUpdated)
}
