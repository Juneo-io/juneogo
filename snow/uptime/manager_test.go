// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package uptime

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
)

var errTest = errors.New("non-nil error")

func TestStartTracking(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	currentTime := startTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestStartTrackingDBError(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.dbWriteError = errTest
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	currentTime := startTime.Add(time.Second)
	clk.Set(currentTime)

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.ErrorIs(err, errTest)
}

func TestStartTrackingNonValidator(t *testing.T) {
	require := require.New(t)

	s := NewTestState()
	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()

	err := up.StartTracking([]ids.NodeID{nodeID0}, supernetID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestStartTrackingInThePast(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	currentTime := startTime.Add(-time.Second)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StopTracking([]ids.NodeID{nodeID0}, supernetID))

	up = NewManager(s, &clk)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestStopTrackingIncreasesUptime(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StopTracking([]ids.NodeID{nodeID0}, supernetID))

	up = NewManager(s, &clk)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestStopTrackingDisconnectedNonValidator(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()

	s := NewTestState()
	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	require.NoError(up.StartTracking(nil, supernetID))

	err := up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestStopTrackingConnectedDBError(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)
	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	require.NoError(up.StartTracking(nil, supernetID))

	require.NoError(up.Connect(nodeID0, supernetID))

	s.dbReadError = errTest
	err := up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.ErrorIs(err, errTest)
}

func TestStopTrackingNonConnectedPast(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)
	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = currentTime.Add(-time.Second)
	clk.Set(currentTime)

	require.NoError(up.StopTracking([]ids.NodeID{nodeID0}, supernetID))

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
	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	s.dbWriteError = errTest
	err := up.StopTracking([]ids.NodeID{nodeID0}, supernetID)
	require.ErrorIs(err, errTest)
}

func TestConnectAndDisconnect(t *testing.T) {
	tests := []struct {
		name      string
		supernetIDs []ids.ID
	}{
		{
			name:      "Single Supernet",
			supernetIDs: []ids.ID{ids.GenerateTestID()},
		},
		{
			name:      "Multiple Supernets",
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
			clk := mockable.Clock{}
			up := NewManager(s, &clk)
			clk.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				s.AddNode(nodeID0, supernetID, startTime)

				connected := up.IsConnected(nodeID0, supernetID)
				require.False(connected)

				require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

				connected = up.IsConnected(nodeID0, supernetID)
				require.False(connected)

				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Duration(0), duration)
				require.Equal(clk.UnixTime(), lastUpdated)

				require.NoError(up.Connect(nodeID0, supernetID))

				connected = up.IsConnected(nodeID0, supernetID)
				require.True(connected)
			}

			currentTime = currentTime.Add(time.Second)
			clk.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Second, duration)
				require.Equal(clk.UnixTime(), lastUpdated)
			}

			require.NoError(up.Disconnect(nodeID0))

			for _, supernetID := range tt.supernetIDs {
				connected := up.IsConnected(nodeID0, supernetID)
				require.False(connected)
			}

			currentTime = currentTime.Add(time.Second)
			clk.Set(currentTime)

			for _, supernetID := range tt.supernetIDs {
				duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
				require.NoError(err)
				require.Equal(time.Second, duration)
				require.Equal(clk.UnixTime(), lastUpdated)
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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.Disconnect(nodeID0))

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)
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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(clk.UnixTime(), lastUpdated)

	require.NoError(up.Connect(nodeID0, supernetID))

	require.NoError(up.Connect(nodeID1, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err = up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)

	require.NoError(up.Disconnect(nodeID1))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err = up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestCalculateUptimeWhenNeverTracked(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	currentTime := startTime.Add(time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)

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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	require.NoError(up.StartTracking([]ids.NodeID{}, supernetID))

	s.AddNode(nodeID0, supernetID, startTime)

	currentTime := startTime.Add(time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(clk.UnixTime(), lastUpdated)

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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(2*time.Second, duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestCalculateUptimeWhenConnectedInFuture(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = currentTime.Add(2 * time.Second)
	clk.Set(currentTime)

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = currentTime.Add(-time.Second)
	clk.Set(currentTime)

	duration, lastUpdated, err := up.CalculateUptime(nodeID0, supernetID)
	require.NoError(err)
	require.Equal(time.Duration(0), duration)
	require.Equal(clk.UnixTime(), lastUpdated)
}

func TestCalculateUptimeNonValidator(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	startTime := time.Now()

	s := NewTestState()

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	_, err := up.CalculateUptimePercentFrom(nodeID0, supernetID, startTime)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestCalculateUptimePercentageDivBy0(t *testing.T) {
	require := require.New(t)

	nodeID0 := ids.GenerateTestNodeID()
	supernetID := ids.GenerateTestID()
	currentTime := time.Now()
	startTime := currentTime

	s := NewTestState()
	s.AddNode(nodeID0, supernetID, startTime)

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = currentTime.Add(time.Second)
	clk.Set(currentTime)

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

	clk := mockable.Clock{}
	up := NewManager(s, &clk)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StopTracking([]ids.NodeID{nodeID0}, supernetID))

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	up = NewManager(s, &clk)

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	require.NoError(up.StartTracking([]ids.NodeID{nodeID0}, supernetID))

	require.NoError(up.Connect(nodeID0, supernetID))

	currentTime = startTime.Add(time.Second)
	clk.Set(currentTime)

	perc, err := up.CalculateUptimePercent(nodeID0, supernetID)
	require.NoError(err)
	require.GreaterOrEqual(float64(1), perc)
}
