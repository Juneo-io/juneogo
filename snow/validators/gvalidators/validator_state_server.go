// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gvalidators

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"

	pb "github.com/Juneo-io/juneogo/proto/pb/validatorstate"
)

var _ pb.ValidatorStateServer = (*Server)(nil)

type Server struct {
	pb.UnsafeValidatorStateServer
	state validators.State
}

func NewServer(state validators.State) *Server {
	return &Server{state: state}
}

func (s *Server) GetMinimumHeight(ctx context.Context, _ *emptypb.Empty) (*pb.GetMinimumHeightResponse, error) {
	height, err := s.state.GetMinimumHeight(ctx)
	return &pb.GetMinimumHeightResponse{Height: height}, err
}

func (s *Server) GetCurrentHeight(ctx context.Context, _ *emptypb.Empty) (*pb.GetCurrentHeightResponse, error) {
	height, err := s.state.GetCurrentHeight(ctx)
	return &pb.GetCurrentHeightResponse{Height: height}, err
}

func (s *Server) GetSupernetID(ctx context.Context, req *pb.GetSupernetIDRequest) (*pb.GetSupernetIDResponse, error) {
	chainID, err := ids.ToID(req.ChainId)
	if err != nil {
		return nil, err
	}

	supernetID, err := s.state.GetSupernetID(ctx, chainID)
	return &pb.GetSupernetIDResponse{
		SupernetId: supernetID[:],
	}, err
}

func (s *Server) GetValidatorSet(ctx context.Context, req *pb.GetValidatorSetRequest) (*pb.GetValidatorSetResponse, error) {
	supernetID, err := ids.ToID(req.SupernetId)
	if err != nil {
		return nil, err
	}

	vdrs, err := s.state.GetValidatorSet(ctx, req.Height, supernetID)
	if err != nil {
		return nil, err
	}

	resp := &pb.GetValidatorSetResponse{
		Validators: make([]*pb.Validator, len(vdrs)),
	}

	i := 0
	for _, vdr := range vdrs {
		vdrPB := &pb.Validator{
			NodeId: vdr.NodeID[:],
			Weight: vdr.Weight,
		}
		if vdr.PublicKey != nil {
			vdrPB.PublicKey = bls.PublicKeyToBytes(vdr.PublicKey)
		}
		resp.Validators[i] = vdrPB
		i++
	}
	return resp, nil
}
