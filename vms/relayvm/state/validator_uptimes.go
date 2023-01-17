// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/relayvm/genesis"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

var _ validatorUptimes = (*uptimes)(nil)

type uptimeAndReward struct {
	UpDuration      time.Duration `serialize:"true"`
	LastUpdated     uint64        `serialize:"true"` // Unix time in seconds
	PotentialReward uint64        `serialize:"true"`

	txID        ids.ID
	lastUpdated time.Time
}

type validatorUptimes interface {
	// LoadUptime sets the uptime measurements of [vdrID] on [supernetID] to
	// [uptime]. GetUptime and SetUptime will return an error if the [vdrID] and
	// [supernetID] hasn't been loaded. This call will not result in a write to disk.
	LoadUptime(
		vdrID ids.NodeID,
		supernetID ids.ID,
		uptime *uptimeAndReward,
	)

	// GetUptime returns the current uptime measurements of [vdrID] on
	// [supernetID].
	GetUptime(
		vdrID ids.NodeID,
		supernetID ids.ID,
	) (upDuration time.Duration, lastUpdated time.Time, err error)

	// SetUptime updates the uptime measurements of [vdrID] on [supernetID].
	// Unless these measurements are deleted first, the next call to
	// WriteUptimes will write this update to disk.
	SetUptime(
		vdrID ids.NodeID,
		supernetID ids.ID,
		upDuration time.Duration,
		lastUpdated time.Time,
	) error

	// DeleteUptime removes in-memory references to the uptimes measurements of
	// [vdrID] on [supernetID]. If there were staged updates from a prior call to
	// SetUptime, the updates will be dropped. This call will not result in a
	// write to disk.
	DeleteUptime(vdrID ids.NodeID, supernetID ids.ID)

	// WriteUptimes writes all staged updates from a prior call to SetUptime.
	WriteUptimes(
		dbPrimary database.KeyValueWriter,
		dbSupernet database.KeyValueWriter,
	) error
}

type uptimes struct {
	uptimes map[ids.NodeID]map[ids.ID]*uptimeAndReward // vdrID -> supernetID -> uptimes
	// updatedUptimes tracks the updates since the last call to WriteUptimes
	updatedUptimes map[ids.NodeID]set.Set[ids.ID] // vdrID -> supernetIDs
}

func newValidatorUptimes() validatorUptimes {
	return &uptimes{
		uptimes:        make(map[ids.NodeID]map[ids.ID]*uptimeAndReward),
		updatedUptimes: make(map[ids.NodeID]set.Set[ids.ID]),
	}
}

func (u *uptimes) LoadUptime(
	vdrID ids.NodeID,
	supernetID ids.ID,
	uptime *uptimeAndReward,
) {
	supernetUptimes, ok := u.uptimes[vdrID]
	if !ok {
		supernetUptimes = make(map[ids.ID]*uptimeAndReward)
		u.uptimes[vdrID] = supernetUptimes
	}
	supernetUptimes[supernetID] = uptime
}

func (u *uptimes) GetUptime(
	vdrID ids.NodeID,
	supernetID ids.ID,
) (upDuration time.Duration, lastUpdated time.Time, err error) {
	uptime, exists := u.uptimes[vdrID][supernetID]
	if !exists {
		return 0, time.Time{}, database.ErrNotFound
	}
	return uptime.UpDuration, uptime.lastUpdated, nil
}

func (u *uptimes) SetUptime(
	vdrID ids.NodeID,
	supernetID ids.ID,
	upDuration time.Duration,
	lastUpdated time.Time,
) error {
	uptime, exists := u.uptimes[vdrID][supernetID]
	if !exists {
		return database.ErrNotFound
	}
	uptime.UpDuration = upDuration
	uptime.lastUpdated = lastUpdated

	updatedSupernetUptimes, ok := u.updatedUptimes[vdrID]
	if !ok {
		updatedSupernetUptimes = set.Set[ids.ID]{}
		u.updatedUptimes[vdrID] = updatedSupernetUptimes
	}
	updatedSupernetUptimes.Add(supernetID)
	return nil
}

func (u *uptimes) DeleteUptime(vdrID ids.NodeID, supernetID ids.ID) {
	supernetUptimes := u.uptimes[vdrID]
	delete(supernetUptimes, supernetID)
	if len(supernetUptimes) == 0 {
		delete(u.uptimes, vdrID)
	}

	supernetUpdatedUptimes := u.updatedUptimes[vdrID]
	delete(supernetUpdatedUptimes, supernetID)
	if len(supernetUpdatedUptimes) == 0 {
		delete(u.updatedUptimes, vdrID)
	}
}

func (u *uptimes) WriteUptimes(
	dbPrimary database.KeyValueWriter,
	dbSupernet database.KeyValueWriter,
) error {
	for vdrID, updatedSupernets := range u.updatedUptimes {
		for supernetID := range updatedSupernets {
			uptime := u.uptimes[vdrID][supernetID]
			uptime.LastUpdated = uint64(uptime.lastUpdated.Unix())

			uptimeBytes, err := genesis.Codec.Marshal(txs.Version, uptime)
			if err != nil {
				return err
			}
			db := dbSupernet
			if supernetID == constants.PrimaryNetworkID {
				db = dbPrimary
			}
			if err := db.Put(uptime.txID[:], uptimeBytes); err != nil {
				return err
			}
		}
		delete(u.updatedUptimes, vdrID)
	}
	return nil
}
