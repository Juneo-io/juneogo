// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package june

import (
	"github.com/Juneo-io/juneogo/database"
)

const (
	IsInitializedKey byte   = iota
	FeesPoolValueKey string = "fees pool value"
)

var (
	isInitializedKey                = []byte{IsInitializedKey}
	feesPoolValueKey                = []byte(FeesPoolValueKey)
	_                SingletonState = (*singletonState)(nil)
)

// SingletonState is a thin wrapper around a database to provide, caching,
// serialization, and de-serialization of singletons.
type SingletonState interface {
	IsInitialized() (bool, error)
	SetInitialized() error
	GetFeesPoolValue() (uint64, error)
	SetFeesPoolValue(fpv uint64) error
}

type singletonState struct {
	singletonDB database.Database
}

func NewSingletonState(db database.Database) SingletonState {
	return &singletonState{
		singletonDB: db,
	}
}

func (s *singletonState) IsInitialized() (bool, error) {
	return s.singletonDB.Has(isInitializedKey)
}

func (s *singletonState) SetInitialized() error {
	return s.singletonDB.Put(isInitializedKey, nil)
}

func (s *singletonState) GetFeesPoolValue() (uint64, error) {
	return database.GetUInt64(s.singletonDB, feesPoolValueKey)
}

func (s *singletonState) SetFeesPoolValue(fpv uint64) error {
	return database.PutUInt64(s.singletonDB, feesPoolValueKey, fpv)
}
