// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package state manages the meta-data required by consensus for an avalanche
// dag.
package state

import (
	"context"
	"errors"
	"time"

	"github.com/Juneo-io/juneogo/cache"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/versiondb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/choices"
	"github.com/Juneo-io/juneogo/snow/consensus/avalanche"
	"github.com/Juneo-io/juneogo/snow/engine/avalanche/vertex"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/utils/set"
)

const (
	dbCacheSize = 10000
	idCacheSize = 1000
)

var (
	errUnknownVertex = errors.New("unknown vertex")
	errWrongChainID  = errors.New("wrong ChainID in vertex")
)

var _ vertex.Manager = (*Serializer)(nil)

// Serializer manages the state of multiple vertices
type Serializer struct {
	SerializerConfig
	versionDB *versiondb.Database
	state     *prefixedState
	edge      set.Set[ids.ID]
}

type SerializerConfig struct {
	ChainID     ids.ID
	VM          vertex.DAGVM
	DB          database.Database
	Log         logging.Logger
	CortinaTime time.Time
}

func NewSerializer(config SerializerConfig) vertex.Manager {
	versionDB := versiondb.New(config.DB)
	dbCache := &cache.LRU[ids.ID, any]{Size: dbCacheSize}
	s := Serializer{
		SerializerConfig: config,
		versionDB:        versionDB,
	}

	rawState := &state{
		serializer: &s,
		log:        config.Log,
		dbCache:    dbCache,
		db:         versionDB,
	}

	s.state = newPrefixedState(rawState, idCacheSize)
	s.edge.Add(s.state.Edge()...)

	return &s
}

func (s *Serializer) ParseVtx(ctx context.Context, b []byte) (avalanche.Vertex, error) {
	return newUniqueVertex(ctx, s, b)
}

func (s *Serializer) BuildStopVtx(
	ctx context.Context,
	parentIDs []ids.ID,
) (avalanche.Vertex, error) {
	height := uint64(0)
	for _, parentID := range parentIDs {
		parent, err := s.getUniqueVertex(parentID)
		if err != nil {
			return nil, err
		}
		parentHeight := parent.v.vtx.Height()
		childHeight, err := math.Add64(parentHeight, 1)
		if err != nil {
			return nil, err
		}
		height = max(height, childHeight)
	}

	vtx, err := vertex.BuildStopVertex(
		s.ChainID,
		height,
		parentIDs,
	)
	if err != nil {
		return nil, err
	}

	uVtx := &uniqueVertex{
		serializer: s,
		id:         vtx.ID(),
	}
	// setVertex handles the case where this vertex already exists even
	// though we just made it
	return uVtx, uVtx.setVertex(ctx, vtx)
}

func (s *Serializer) GetVtx(_ context.Context, vtxID ids.ID) (avalanche.Vertex, error) {
	return s.getUniqueVertex(vtxID)
}

func (s *Serializer) Edge(context.Context) []ids.ID {
	return s.edge.List()
}

func (s *Serializer) parseVertex(b []byte) (vertex.StatelessVertex, error) {
	vtx, err := vertex.Parse(b)
	if err != nil {
		return nil, err
	}
	if vtx.ChainID() != s.ChainID {
		return nil, errWrongChainID
	}
	return vtx, nil
}

func (s *Serializer) getUniqueVertex(vtxID ids.ID) (*uniqueVertex, error) {
	vtx := &uniqueVertex{
		serializer: s,
		id:         vtxID,
	}
	if vtx.Status() == choices.Unknown {
		return nil, errUnknownVertex
	}
	return vtx, nil
}

func (s *Serializer) StopVertexAccepted(ctx context.Context) (bool, error) {
	edge := s.Edge(ctx)
	if len(edge) != 1 {
		return false, nil
	}

	vtx, err := s.getUniqueVertex(edge[0])
	if err != nil {
		return false, err
	}

	return vtx.v.vtx.StopVertex(), nil
}
