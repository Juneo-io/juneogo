// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gvalidators

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/vms/rpcchainvm/grpcutils"

	pb "github.com/Juneo-io/juneogo/proto/pb/validatorstate"
)

const bufSize = 1024 * 1024

var errCustom = errors.New("custom")

type testState struct {
	client  *Client
	server  *validators.MockState
	closeFn func()
}

func setupState(t testing.TB, ctrl *gomock.Controller) *testState {
	t.Helper()

	state := &testState{
		server: validators.NewMockState(ctrl),
	}

	listener := bufconn.Listen(bufSize)
	serverCloser := grpcutils.ServerCloser{}

	serverFunc := func(opts []grpc.ServerOption) *grpc.Server {
		server := grpcutils.NewDefaultServer(opts)
		pb.RegisterValidatorStateServer(server, NewServer(state.server))
		serverCloser.Add(server)
		return server
	}

	go grpcutils.Serve(listener, serverFunc)

	dialer := grpc.WithContextDialer(
		func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		},
	)

	dopts := grpcutils.DefaultDialOptions
	dopts = append(dopts, dialer)
	conn, err := grpcutils.Dial("", dopts...)
	if err != nil {
		t.Fatalf("Failed to dial: %s", err)
	}

	state.client = NewClient(pb.NewValidatorStateClient(conn))
	state.closeFn = func() {
		serverCloser.Stop()
		_ = conn.Close()
		_ = listener.Close()
	}
	return state
}

func TestGetMinimumHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := setupState(t, ctrl)
	defer state.closeFn()

	// Happy path
	expectedHeight := uint64(1337)
	state.server.EXPECT().GetMinimumHeight(gomock.Any()).Return(expectedHeight, nil)

	height, err := state.client.GetMinimumHeight(context.Background())
	require.NoError(err)
	require.Equal(expectedHeight, height)

	// Error path
	state.server.EXPECT().GetMinimumHeight(gomock.Any()).Return(expectedHeight, errCustom)

	_, err = state.client.GetMinimumHeight(context.Background())
	require.Error(err)
}

func TestGetCurrentHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := setupState(t, ctrl)
	defer state.closeFn()

	// Happy path
	expectedHeight := uint64(1337)
	state.server.EXPECT().GetCurrentHeight(gomock.Any()).Return(expectedHeight, nil)

	height, err := state.client.GetCurrentHeight(context.Background())
	require.NoError(err)
	require.Equal(expectedHeight, height)

	// Error path
	state.server.EXPECT().GetCurrentHeight(gomock.Any()).Return(expectedHeight, errCustom)

	_, err = state.client.GetCurrentHeight(context.Background())
	require.Error(err)
}

func TestGetSupernetID(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := setupState(t, ctrl)
	defer state.closeFn()

	// Happy path
	chainID := ids.GenerateTestID()
	expectedSupernetID := ids.GenerateTestID()
	state.server.EXPECT().GetSupernetID(gomock.Any(), chainID).Return(expectedSupernetID, nil)

	supernetID, err := state.client.GetSupernetID(context.Background(), chainID)
	require.NoError(err)
	require.Equal(expectedSupernetID, supernetID)

	// Error path
	state.server.EXPECT().GetSupernetID(gomock.Any(), chainID).Return(expectedSupernetID, errCustom)

	_, err = state.client.GetSupernetID(context.Background(), chainID)
	require.Error(err)
}

func TestGetValidatorSet(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	state := setupState(t, ctrl)
	defer state.closeFn()

	// Happy path
	sk0, err := bls.NewSecretKey()
	require.NoError(err)
	vdr0 := &validators.GetValidatorOutput{
		NodeID:    ids.GenerateTestNodeID(),
		PublicKey: bls.PublicFromSecretKey(sk0),
		Weight:    1,
	}

	sk1, err := bls.NewSecretKey()
	require.NoError(err)
	vdr1 := &validators.GetValidatorOutput{
		NodeID:    ids.GenerateTestNodeID(),
		PublicKey: bls.PublicFromSecretKey(sk1),
		Weight:    2,
	}

	vdr2 := &validators.GetValidatorOutput{
		NodeID:    ids.GenerateTestNodeID(),
		PublicKey: nil,
		Weight:    3,
	}

	expectedVdrs := map[ids.NodeID]*validators.GetValidatorOutput{
		vdr0.NodeID: vdr0,
		vdr1.NodeID: vdr1,
		vdr2.NodeID: vdr2,
	}
	height := uint64(1337)
	supernetID := ids.GenerateTestID()
	state.server.EXPECT().GetValidatorSet(gomock.Any(), height, supernetID).Return(expectedVdrs, nil)

	vdrs, err := state.client.GetValidatorSet(context.Background(), height, supernetID)
	require.NoError(err)
	require.Equal(expectedVdrs, vdrs)

	// Error path
	state.server.EXPECT().GetValidatorSet(gomock.Any(), height, supernetID).Return(expectedVdrs, errCustom)

	_, err = state.client.GetValidatorSet(context.Background(), height, supernetID)
	require.Error(err)
}
