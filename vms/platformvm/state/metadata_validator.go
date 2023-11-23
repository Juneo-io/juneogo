// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"time"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/wrappers"
)

// preDelegateeRewardSize is the size of codec marshalling
// [preDelegateeRewardMetadata].
//
// CodecVersionLen + UpDurationLen + LastUpdatedLen + PotentialRewardLen
const preDelegateeRewardSize = wrappers.ShortLen + 3*wrappers.LongLen

var _ validatorState = (*metadata)(nil)

type preDelegateeRewardMetadata struct {
	UpDuration      time.Duration `v0:"true"`
	LastUpdated     uint64        `v0:"true"` // Unix time in seconds
	PotentialReward uint64        `v0:"true"`
}

type validatorMetadata struct {
	UpDuration               time.Duration `v0:"true"`
	LastUpdated              uint64        `v0:"true"` // Unix time in seconds
	PotentialReward          uint64        `v0:"true"`
	PotentialDelegateeReward uint64        `v0:"true"`

	txID        ids.ID
	lastUpdated time.Time
}

// Permissioned validators originally wrote their values as nil.
// With Banff we wrote the potential reward.
// With Cortina we wrote the potential reward with the potential delegatee reward.
// We now write the uptime, reward, and delegatee reward together.
func parseValidatorMetadata(bytes []byte, metadata *validatorMetadata) error {
	switch len(bytes) {
	case 0:
	// nothing was stored

	case database.Uint64Size:
		// only potential reward was stored
		var err error
		metadata.PotentialReward, err = database.ParseUInt64(bytes)
		if err != nil {
			return err
		}

	case preDelegateeRewardSize:
		// potential reward and uptime was stored but potential delegatee reward
		// was not
		tmp := preDelegateeRewardMetadata{}
		if _, err := metadataCodec.Unmarshal(bytes, &tmp); err != nil {
			return err
		}

		metadata.UpDuration = tmp.UpDuration
		metadata.LastUpdated = tmp.LastUpdated
		metadata.PotentialReward = tmp.PotentialReward
	default:
		// everything was stored
		if _, err := metadataCodec.Unmarshal(bytes, metadata); err != nil {
			return err
		}
	}
	metadata.lastUpdated = time.Unix(int64(metadata.LastUpdated), 0)
	return nil
}

type validatorState interface {
	// LoadValidatorMetadata sets the [metadata] of [vdrID] on [supernetID].
	// GetUptime and SetUptime will return an error if the [vdrID] and
	// [supernetID] hasn't been loaded. This call will not result in a write to
	// disk.
	LoadValidatorMetadata(
		vdrID ids.NodeID,
		supernetID ids.ID,
		metadata *validatorMetadata,
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

	// GetDelegateeReward returns the current rewards accrued to [vdrID] on
	// [supernetID].
	GetDelegateeReward(
		supernetID ids.ID,
		vdrID ids.NodeID,
	) (amount uint64, err error)

	// SetDelegateeReward updates the rewards accrued to [vdrID] on [supernetID].
	// Unless these measurements are deleted first, the next call to
	// WriteUptimes will write this update to disk.
	SetDelegateeReward(
		supernetID ids.ID,
		vdrID ids.NodeID,
		amount uint64,
	) error

	// DeleteValidatorMetadata removes in-memory references to the metadata of
	// [vdrID] on [supernetID]. If there were staged updates from a prior call to
	// SetUptime or SetDelegateeReward, the updates will be dropped. This call
	// will not result in a write to disk.
	DeleteValidatorMetadata(vdrID ids.NodeID, supernetID ids.ID)

	// WriteValidatorMetadata writes all staged updates from prior calls to
	// SetUptime or SetDelegateeReward.
	WriteValidatorMetadata(
		dbPrimary database.KeyValueWriter,
		dbSupernet database.KeyValueWriter,
	) error
}

type metadata struct {
	metadata map[ids.NodeID]map[ids.ID]*validatorMetadata // vdrID -> supernetID -> metadata
	// updatedMetadata tracks the updates since WriteValidatorMetadata was last called
	updatedMetadata map[ids.NodeID]set.Set[ids.ID] // vdrID -> supernetIDs
}

func newValidatorState() validatorState {
	return &metadata{
		metadata:        make(map[ids.NodeID]map[ids.ID]*validatorMetadata),
		updatedMetadata: make(map[ids.NodeID]set.Set[ids.ID]),
	}
}

func (m *metadata) LoadValidatorMetadata(
	vdrID ids.NodeID,
	supernetID ids.ID,
	uptime *validatorMetadata,
) {
	supernetMetadata, ok := m.metadata[vdrID]
	if !ok {
		supernetMetadata = make(map[ids.ID]*validatorMetadata)
		m.metadata[vdrID] = supernetMetadata
	}
	supernetMetadata[supernetID] = uptime
}

func (m *metadata) GetUptime(
	vdrID ids.NodeID,
	supernetID ids.ID,
) (time.Duration, time.Time, error) {
	metadata, exists := m.metadata[vdrID][supernetID]
	if !exists {
		return 0, time.Time{}, database.ErrNotFound
	}
	return metadata.UpDuration, metadata.lastUpdated, nil
}

func (m *metadata) SetUptime(
	vdrID ids.NodeID,
	supernetID ids.ID,
	upDuration time.Duration,
	lastUpdated time.Time,
) error {
	metadata, exists := m.metadata[vdrID][supernetID]
	if !exists {
		return database.ErrNotFound
	}
	metadata.UpDuration = upDuration
	metadata.lastUpdated = lastUpdated

	m.addUpdatedMetadata(vdrID, supernetID)
	return nil
}

func (m *metadata) GetDelegateeReward(
	supernetID ids.ID,
	vdrID ids.NodeID,
) (uint64, error) {
	metadata, exists := m.metadata[vdrID][supernetID]
	if !exists {
		return 0, database.ErrNotFound
	}
	return metadata.PotentialDelegateeReward, nil
}

func (m *metadata) SetDelegateeReward(
	supernetID ids.ID,
	vdrID ids.NodeID,
	amount uint64,
) error {
	metadata, exists := m.metadata[vdrID][supernetID]
	if !exists {
		return database.ErrNotFound
	}
	metadata.PotentialDelegateeReward = amount

	m.addUpdatedMetadata(vdrID, supernetID)
	return nil
}

func (m *metadata) DeleteValidatorMetadata(vdrID ids.NodeID, supernetID ids.ID) {
	supernetMetadata := m.metadata[vdrID]
	delete(supernetMetadata, supernetID)
	if len(supernetMetadata) == 0 {
		delete(m.metadata, vdrID)
	}

	supernetUpdatedMetadata := m.updatedMetadata[vdrID]
	supernetUpdatedMetadata.Remove(supernetID)
	if supernetUpdatedMetadata.Len() == 0 {
		delete(m.updatedMetadata, vdrID)
	}
}

func (m *metadata) WriteValidatorMetadata(
	dbPrimary database.KeyValueWriter,
	dbSupernet database.KeyValueWriter,
) error {
	for vdrID, updatedSupernets := range m.updatedMetadata {
		for supernetID := range updatedSupernets {
			metadata := m.metadata[vdrID][supernetID]
			metadata.LastUpdated = uint64(metadata.lastUpdated.Unix())

			metadataBytes, err := metadataCodec.Marshal(v0, metadata)
			if err != nil {
				return err
			}
			db := dbSupernet
			if supernetID == constants.PrimaryNetworkID {
				db = dbPrimary
			}
			if err := db.Put(metadata.txID[:], metadataBytes); err != nil {
				return err
			}
		}
		delete(m.updatedMetadata, vdrID)
	}
	return nil
}

func (m *metadata) addUpdatedMetadata(vdrID ids.NodeID, supernetID ids.ID) {
	updatedSupernetMetadata, ok := m.updatedMetadata[vdrID]
	if !ok {
		updatedSupernetMetadata = set.Set[ids.ID]{}
		m.updatedMetadata[vdrID] = updatedSupernetMetadata
	}
	updatedSupernetMetadata.Add(supernetID)
}
