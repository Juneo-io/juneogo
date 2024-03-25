// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chains

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/utils/constants"
)

func TestNewSupernets(t *testing.T) {
	require := require.New(t)
	config := map[ids.ID]supernets.Config{
		constants.PrimaryNetworkID: {},
	}

	supernets, err := NewSupernets(ids.EmptyNodeID, config)
	require.NoError(err)

	supernet, ok := supernets.GetOrCreate(constants.PrimaryNetworkID)
	require.False(ok)
	require.Equal(config[constants.PrimaryNetworkID], supernet.Config())
}

func TestNewSupernetsNoPrimaryNetworkConfig(t *testing.T) {
	require := require.New(t)
	config := map[ids.ID]supernets.Config{}

	_, err := NewSupernets(ids.EmptyNodeID, config)
	require.ErrorIs(err, ErrNoPrimaryNetworkConfig)
}

func TestSupernetsGetOrCreate(t *testing.T) {
	testSupernetID := ids.GenerateTestID()

	type args struct {
		supernetID ids.ID
		want     bool
	}

	tests := []struct {
		name string
		args []args
	}{
		{
			name: "adding duplicate supernet is a noop",
			args: []args{
				{
					supernetID: testSupernetID,
					want:     true,
				},
				{
					supernetID: testSupernetID,
				},
			},
		},
		{
			name: "adding unique supernets succeeds",
			args: []args{
				{
					supernetID: ids.GenerateTestID(),
					want:     true,
				},
				{
					supernetID: ids.GenerateTestID(),
					want:     true,
				},
				{
					supernetID: ids.GenerateTestID(),
					want:     true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			config := map[ids.ID]supernets.Config{
				constants.PrimaryNetworkID: {},
			}
			supernets, err := NewSupernets(ids.EmptyNodeID, config)
			require.NoError(err)

			for _, arg := range tt.args {
				_, got := supernets.GetOrCreate(arg.supernetID)
				require.Equal(arg.want, got)
			}
		})
	}
}

func TestSupernetConfigs(t *testing.T) {
	testSupernetID := ids.GenerateTestID()

	tests := []struct {
		name     string
		config   map[ids.ID]supernets.Config
		supernetID ids.ID
		want     supernets.Config
	}{
		{
			name: "default to primary network config",
			config: map[ids.ID]supernets.Config{
				constants.PrimaryNetworkID: {},
			},
			supernetID: testSupernetID,
			want:     supernets.Config{},
		},
		{
			name: "use supernet config",
			config: map[ids.ID]supernets.Config{
				constants.PrimaryNetworkID: {},
				testSupernetID: {
					GossipConfig: supernets.GossipConfig{
						AcceptedFrontierValidatorSize: 123456789,
					},
				},
			},
			supernetID: testSupernetID,
			want: supernets.Config{
				GossipConfig: supernets.GossipConfig{
					AcceptedFrontierValidatorSize: 123456789,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			supernets, err := NewSupernets(ids.EmptyNodeID, tt.config)
			require.NoError(err)

			supernet, ok := supernets.GetOrCreate(tt.supernetID)
			require.True(ok)

			require.Equal(tt.want, supernet.Config())
		})
	}
}

func TestSupernetsBootstrapping(t *testing.T) {
	require := require.New(t)

	config := map[ids.ID]supernets.Config{
		constants.PrimaryNetworkID: {},
	}

	supernets, err := NewSupernets(ids.EmptyNodeID, config)
	require.NoError(err)

	supernetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	supernet, ok := supernets.GetOrCreate(supernetID)
	require.True(ok)

	// Start bootstrapping
	supernet.AddChain(chainID)
	bootstrapping := supernets.Bootstrapping()
	require.Contains(bootstrapping, supernetID)

	// Finish bootstrapping
	supernet.Bootstrapped(chainID)
	require.Empty(supernets.Bootstrapping())
}
