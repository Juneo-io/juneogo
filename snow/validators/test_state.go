// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
)

var (
	errMinimumHeight   = errors.New("unexpectedly called GetMinimumHeight")
	errCurrentHeight   = errors.New("unexpectedly called GetCurrentHeight")
	errSupernetID        = errors.New("unexpectedly called GetSupernetID")
	errGetValidatorSet = errors.New("unexpectedly called GetValidatorSet")
)

var _ State = (*TestState)(nil)

type TestState struct {
	T testing.TB

	CantGetMinimumHeight,
	CantGetCurrentHeight,
	CantGetSupernetID,
	CantGetValidatorSet bool

	GetMinimumHeightF func(ctx context.Context) (uint64, error)
	GetCurrentHeightF func(ctx context.Context) (uint64, error)
	GetSupernetIDF      func(ctx context.Context, chainID ids.ID) (ids.ID, error)
	GetValidatorSetF  func(ctx context.Context, height uint64, supernetID ids.ID) (map[ids.NodeID]*GetValidatorOutput, error)
}

func (vm *TestState) GetMinimumHeight(ctx context.Context) (uint64, error) {
	if vm.GetMinimumHeightF != nil {
		return vm.GetMinimumHeightF(ctx)
	}
	if vm.CantGetMinimumHeight && vm.T != nil {
		require.FailNow(vm.T, errMinimumHeight.Error())
	}
	return 0, errMinimumHeight
}

func (vm *TestState) GetCurrentHeight(ctx context.Context) (uint64, error) {
	if vm.GetCurrentHeightF != nil {
		return vm.GetCurrentHeightF(ctx)
	}
	if vm.CantGetCurrentHeight && vm.T != nil {
		require.FailNow(vm.T, errCurrentHeight.Error())
	}
	return 0, errCurrentHeight
}

func (vm *TestState) GetSupernetID(ctx context.Context, chainID ids.ID) (ids.ID, error) {
	if vm.GetSupernetIDF != nil {
		return vm.GetSupernetIDF(ctx, chainID)
	}
	if vm.CantGetSupernetID && vm.T != nil {
		require.FailNow(vm.T, errSupernetID.Error())
	}
	return ids.Empty, errSupernetID
}

func (vm *TestState) GetValidatorSet(
	ctx context.Context,
	height uint64,
	supernetID ids.ID,
) (map[ids.NodeID]*GetValidatorOutput, error) {
	if vm.GetValidatorSetF != nil {
		return vm.GetValidatorSetF(ctx, height, supernetID)
	}
	if vm.CantGetValidatorSet && vm.T != nil {
		require.FailNow(vm.T, errGetValidatorSet.Error())
	}
	return nil, errGetValidatorSet
}
