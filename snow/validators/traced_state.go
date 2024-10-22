// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/trace"

	oteltrace "go.opentelemetry.io/otel/trace"
)

var _ State = (*tracedState)(nil)

type tracedState struct {
	s                   State
	getMinimumHeightTag string
	getCurrentHeightTag string
	getSupernetIDTag      string
	getValidatorSetTag  string
	tracer              trace.Tracer
}

func Trace(s State, name string, tracer trace.Tracer) State {
	return &tracedState{
		s:                   s,
		getMinimumHeightTag: name + ".GetMinimumHeight",
		getCurrentHeightTag: name + ".GetCurrentHeight",
		getSupernetIDTag:      name + ".GetSupernetID",
		getValidatorSetTag:  name + ".GetValidatorSet",
		tracer:              tracer,
	}
}

func (s *tracedState) GetMinimumHeight(ctx context.Context) (uint64, error) {
	ctx, span := s.tracer.Start(ctx, s.getMinimumHeightTag)
	defer span.End()

	return s.s.GetMinimumHeight(ctx)
}

func (s *tracedState) GetCurrentHeight(ctx context.Context) (uint64, error) {
	ctx, span := s.tracer.Start(ctx, s.getCurrentHeightTag)
	defer span.End()

	return s.s.GetCurrentHeight(ctx)
}

func (s *tracedState) GetSupernetID(ctx context.Context, chainID ids.ID) (ids.ID, error) {
	ctx, span := s.tracer.Start(ctx, s.getValidatorSetTag, oteltrace.WithAttributes(
		attribute.Stringer("chainID", chainID),
	))
	defer span.End()

	return s.s.GetSupernetID(ctx, chainID)
}

func (s *tracedState) GetValidatorSet(
	ctx context.Context,
	height uint64,
	supernetID ids.ID,
) (map[ids.NodeID]*GetValidatorOutput, error) {
	ctx, span := s.tracer.Start(ctx, s.getValidatorSetTag, oteltrace.WithAttributes(
		attribute.Int64("height", int64(height)),
		attribute.Stringer("supernetID", supernetID),
	))
	defer span.End()

	return s.s.GetValidatorSet(ctx, height, supernetID)
}
