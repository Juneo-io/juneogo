// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package uptime

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
)

var errTest = errors.New("non-nil error")

func TestStartTracking(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)

	currentTime := startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestStartTrackingDBError(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.dbWriteError = errTest
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)

	currentTime := startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.Error(err)
}

func TestStartTrackingNonValidator(t *testing.T) {
	require := require.New(t)

	s := NewTestState()
	up := NewManager(s).(*manager)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.Error(err)
}

func TestStartTrackingInThePast(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)

	currentTime := startTime.Add(-time.Second)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(startTime.Truncate(time.Second), lastUpdated)
}

func TestStopTrackingDecreasesUptime(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	up = NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err = up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestStopTrackingIncreasesUptime(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	up = NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err = up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestStopTrackingDisconnectedNonValidator(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()

	s := NewTestState()
	up := NewManager(s).(*manager)

	err := up.StartTracking(nil, supernetID)
	require.NoError(err)

	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.Error(err)
}

func TestStopTrackingConnectedDBError(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)
	up := NewManager(s).(*manager)

	err := up.StartTracking(nil, supernetID)
	require.NoError(err)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	s.dbReadError = errTest
	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.Error(err)
}

func TestStopTrackingNonConnectedPast(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)
	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(-time.Second)
	up.clock.Set(currentTime)

	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := s.GetUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(startTime.Truncate(time.Second), lastUpdated)
}

func TestStopTrackingNonConnectedDBError(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)
	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	s.dbWriteError = errTest
	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.Error(err)
}

func TestConnectAndDisconnect(t *testing.T) {
	tests := []struct {
		name        string
		supernetIDs []ids.ID
	}{
		{
			name:        "Single Supernet",
			supernetIDs: []ids.ID{ids.GenerateTestID()},
		},
		{
			name:        "Multiple Supernets",
			supernetIDs: []ids.ID{ids.GenerateTestID(), ids.GenerateTestID()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			nodeID0 := ids.GenerateTestNodeID()
			currentTime := time.Now()
			startTime := currentTime

			s := NewTestState()
			up := NewManager(s).(*manager)
			up.clock.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				s.AddNode(nodeID0, supernetID, startTime)

				connected := up.IsConnected(nodeID0, supernetID)
				require.False(connected)

				err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
				require.NoError(err)

				connected = up.IsConnected(nodeID0, supernetID)
				require.False(connected)

				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Duration(0), duration)
				require.Equal(up.clock.UnixTime(), lastUpdated)

				err = up.Connect(nodeID0, supernetID)
				require.NoError(err)

				connected = up.IsConnected(nodeID0, supernetID)
				require.True(connected)
			}

			currentTime = currentTime.Add(time.Second)
			up.clock.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Second, duration)
				require.Equal(up.clock.UnixTime(), lastUpdated)
			}

			err := up.Disconnect(nodeID0)
			require.NoError(err)

			for _, supernetID := range tt.supernetIDs {
				connected := up.IsConnected(nodeID0, supernetID)
				require.False(connected)
			}

			currentTime = currentTime.Add(time.Second)
			up.clock.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Second, duration)
				require.Equal(up.clock.UnixTime(), lastUpdated)
			}
		})
	}
}

func TestConnectAndDisconnectBeforeTracking(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	err := up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.Disconnect(nodeID0)
	require.NoError(err)

	err = up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestUnrelatedNodeDisconnect(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	nodeID1 := ids.GenerateTestNodeID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	err = up.Connect(nodeID1, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err = up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)

	err = up.Disconnect(nodeID1)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err = up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestCalculateUptimeWhenNeverTracked(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)

	currentTime := startTime.Add(time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)

	uptime, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime.Truncate(time.Second))
	require.NoError(err)
	require.Equal(float64(1), uptime)
}

func TestCalculateUptimeWhenNeverConnected(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()

	up := NewManager(s).(*manager)

	err := up.StartTracking([]ids.NodeID{}, supernetID)
	require.NoError(err)

	s.AddNode(nodeID0, supernetID, startTime)

	currentTime := startTime.Add(time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)

	uptime, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime)
	require.NoError(err)
	require.Equal(float64(0), uptime)
}

func TestCalculateUptimeWhenConnectedBeforeTracking(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestCalculateUptimeWhenConnectedInFuture(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(2 * time.Second)
	up.clock.Set(currentTime)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(-time.Second)
	up.clock.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(up.clock.UnixTime(), lastUpdated)
}

func TestCalculateUptimeNonValidator(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()

	up := NewManager(s).(*manager)

	_, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime)
	require.Error(err)
}

func TestCalculateUptimePercentageDivBy0(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	uptime, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime.Truncate(time.Second))
	require.NoError(err)
	require.Equal(float64(1), uptime)
}

func TestCalculateUptimePercentage(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	currentTime := time.Now()
	supernetID := ids.GenerateTestID()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = currentTime.Add(time.Second)
	up.clock.Set(currentTime)

	uptime, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime.Truncate(time.Second))
	require.NoError(err)
	require.Equal(float64(0), uptime)
}

func TestStopTrackingUnixTimeRegression(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	currentTime := time.Now()
	supernetID := ids.GenerateTestID()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	up := NewManager(s).(*manager)
	up.clock.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	up = NewManager(s).(*manager)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	err = up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.NoError(err)

	err = up.Connect(nodeID0, supernetID)
	require.NoError(err)

	currentTime = startTime.Add(time.Second)
	up.clock.Set(currentTime)

	perc, err := up.CalculateUptimePercent(nodeID0, supernetID)
	require.NoError(err)
	require.GreaterOrEqual(float64(1), perc)
}
