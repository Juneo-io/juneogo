// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"errors"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
)

var errInvalidETHAddress = errors.New("invalid eth address")

type UnparsedAllocation struct {
	ETHAddr        string         `json:"ethAddr"`
	AVAXAddr       string         `json:"avaxAddr"`
	InitialAmount  uint64         `json:"initialAmount"`
	UnlockSchedule []LockedAmount `json:"unlockSchedule"`
}

func (ua UnparsedAllocation) Parse() (Allocation, error) {
	a := Allocation{
		InitialAmount:  ua.InitialAmount,
		UnlockSchedule: ua.UnlockSchedule,
	}

	if len(ua.ETHAddr) < 2 {
		return a, errInvalidETHAddress
	}

	ethAddrBytes, err := hex.DecodeString(ua.ETHAddr[2:])
	if err != nil {
		return a, err
	}
	ethAddr, err := ids.ToShortID(ethAddrBytes)
	if err != nil {
		return a, err
	}
	a.ETHAddr = ethAddr

	_, _, avaxAddrBytes, err := address.Parse(ua.AVAXAddr)
	if err != nil {
		return a, err
	}
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	if err != nil {
		return a, err
	}
	a.AVAXAddr = avaxAddr

	return a, nil
}

type UnparsedStaker struct {
	NodeID        ids.NodeID                `json:"nodeID"`
	RewardAddress string                    `json:"rewardAddress"`
	DelegationFee uint32                    `json:"delegationFee"`
	Signer        *signer.ProofOfPossession `json:"signer,omitempty"`
}

func (us UnparsedStaker) Parse() (Staker, error) {
	s := Staker{
		NodeID:        us.NodeID,
		DelegationFee: us.DelegationFee,
		Signer:        us.Signer,
	}

	_, _, avaxAddrBytes, err := address.Parse(us.RewardAddress)
	if err != nil {
		return s, err
	}
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	if err != nil {
		return s, err
	}
	s.RewardAddress = avaxAddr
	return s, nil
}

// UnparsedConfig contains the genesis addresses used to construct a genesis
type UnparsedConfig struct {
	NetworkID        uint32 `json:"networkID"`
	RewardPoolSupply uint64 `json:"rewardPoolSupply"`

	Allocations []UnparsedAllocation `json:"allocations"`

	StartTime                  uint64           `json:"startTime"`
	InitialStakeDuration       uint64           `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64           `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string         `json:"initialStakedFunds"`
	InitialStakers             []UnparsedStaker `json:"initialStakers"`

	JUNEChainGenesis  string `json:"JUNEChainGenesis"`
	USDT1ChainGenesis string `json:"USDT1ChainGenesis"`
	USD1ChainGenesis  string `json:"USD1ChainGenesis"`
	DAI1ChainGenesis  string `json:"DAI1ChainGenesis"`
	EUR1ChainGenesis  string `json:"EUR1ChainGenesis"`
	SGD1ChainGenesis  string `json:"SGD1ChainGenesis"`
	GLD1ChainGenesis  string `json:"GLD1ChainGenesis"`
	MBTC1ChainGenesis string `json:"MBTC1ChainGenesis"`
	DOGE1ChainGenesis string `json:"DOGE1ChainGenesis"`
	LTC1ChainGenesis  string `json:"LTC1ChainGenesis"`

	Message string `json:"message"`
}

func (uc UnparsedConfig) Parse() (Config, error) {
	c := Config{
		NetworkID:                  uc.NetworkID,
		RewardPoolSupply:           uc.RewardPoolSupply,
		Allocations:                make([]Allocation, len(uc.Allocations)),
		StartTime:                  uc.StartTime,
		InitialStakeDuration:       uc.InitialStakeDuration,
		InitialStakeDurationOffset: uc.InitialStakeDurationOffset,
		InitialStakedFunds:         make([]ids.ShortID, len(uc.InitialStakedFunds)),
		InitialStakers:             make([]Staker, len(uc.InitialStakers)),
		JUNEChainGenesis:           uc.JUNEChainGenesis,
		USDT1ChainGenesis:          uc.USDT1ChainGenesis,
		USD1ChainGenesis:           uc.USD1ChainGenesis,
		DAI1ChainGenesis:           uc.DAI1ChainGenesis,
		EUR1ChainGenesis:           uc.EUR1ChainGenesis,
		SGD1ChainGenesis:           uc.SGD1ChainGenesis,
		GLD1ChainGenesis:           uc.GLD1ChainGenesis,
		MBTC1ChainGenesis:          uc.MBTC1ChainGenesis,
		DOGE1ChainGenesis:          uc.DOGE1ChainGenesis,
		LTC1ChainGenesis:           uc.LTC1ChainGenesis,
		Message:                    uc.Message,
	}
	for i, ua := range uc.Allocations {
		a, err := ua.Parse()
		if err != nil {
			return c, err
		}
		c.Allocations[i] = a
	}
	for i, isa := range uc.InitialStakedFunds {
		_, _, avaxAddrBytes, err := address.Parse(isa)
		if err != nil {
			return c, err
		}
		avaxAddr, err := ids.ToShortID(avaxAddrBytes)
		if err != nil {
			return c, err
		}
		c.InitialStakedFunds[i] = avaxAddr
	}
	for i, uis := range uc.InitialStakers {
		is, err := uis.Parse()
		if err != nil {
			return c, err
		}
		c.InitialStakers[i] = is
	}
	return c, nil
}
