// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"errors"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
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
	NodeID        ids.NodeID `json:"nodeID"`
	RewardAddress string     `json:"rewardAddress"`
	DelegationFee uint32     `json:"delegationFee"`
}

func (us UnparsedStaker) Parse() (Staker, error) {
	s := Staker{
		NodeID:        us.NodeID,
		DelegationFee: us.DelegationFee,
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
	NetworkID         uint32 `json:"networkID"`
	RewardsPoolSupply uint64 `json:"rewardsPoolSupply"`

	Allocations []UnparsedAllocation `json:"allocations"`

	StartTime                  uint64           `json:"startTime"`
	InitialStakeDuration       uint64           `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64           `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string         `json:"initialStakedFunds"`
	InitialStakers             []UnparsedStaker `json:"initialStakers"`

	JUNEChainGenesis   string `json:"JUNEChainGenesis"`
	ETH1ChainGenesis   string `json:"ETH1ChainGenesis"`
	MBTC1ChainGenesis  string `json:"MBTC1ChainGenesis"`
	DOGE1ChainGenesis  string `json:"DOGE1ChainGenesis"`
	TUSD1ChainGenesis  string `json:"TUSD1ChainGenesis"`
	USDT1ChainGenesis  string `json:"USDT1ChainGenesis"`
	DAI1ChainGenesis   string `json:"DAI1ChainGenesis"`
	EUROC1ChainGenesis string `json:"EUROC1ChainGenesis"`
	LTC1ChainGenesis   string `json:"LTC1ChainGenesis"`
	XLM1ChainGenesis   string `json:"XLM1ChainGenesis"`
	BCH1ChainGenesis   string `json:"BCH1ChainGenesis"`
	PAXG1ChainGenesis  string `json:"PAXG1ChainGenesis"`
	ICP1ChainGenesis   string `json:"ICP1ChainGenesis"`
	XIDR1ChainGenesis  string `json:"XIDR1ChainGenesis"`
	XSGD1ChainGenesis  string `json:"XSGD1ChainGenesis"`
	ETC1ChainGenesis   string `json:"ETC1ChainGenesis"`
	R1000ChainGenesis  string `json:"R1000ChainGenesis"`
	R10ChainGenesis    string `json:"R10ChainGenesis"`

	Message string `json:"message"`
}

func (uc UnparsedConfig) Parse() (Config, error) {
	c := Config{
		NetworkID:                  uc.NetworkID,
		RewardsPoolSupply:          uc.RewardsPoolSupply,
		Allocations:                make([]Allocation, len(uc.Allocations)),
		StartTime:                  uc.StartTime,
		InitialStakeDuration:       uc.InitialStakeDuration,
		InitialStakeDurationOffset: uc.InitialStakeDurationOffset,
		InitialStakedFunds:         make([]ids.ShortID, len(uc.InitialStakedFunds)),
		InitialStakers:             make([]Staker, len(uc.InitialStakers)),
		JUNEChainGenesis:           uc.JUNEChainGenesis,
		ETH1ChainGenesis:           uc.ETH1ChainGenesis,
		MBTC1ChainGenesis:          uc.MBTC1ChainGenesis,
		DOGE1ChainGenesis:          uc.DOGE1ChainGenesis,
		TUSD1ChainGenesis:          uc.TUSD1ChainGenesis,
		USDT1ChainGenesis:          uc.USDT1ChainGenesis,
		DAI1ChainGenesis:           uc.DAI1ChainGenesis,
		EUROC1ChainGenesis:         uc.EUROC1ChainGenesis,
		LTC1ChainGenesis:           uc.LTC1ChainGenesis,
		XLM1ChainGenesis:           uc.XLM1ChainGenesis,
		BCH1ChainGenesis:           uc.BCH1ChainGenesis,
		PAXG1ChainGenesis:          uc.PAXG1ChainGenesis,
		ICP1ChainGenesis:           uc.ICP1ChainGenesis,
		XIDR1ChainGenesis:          uc.XIDR1ChainGenesis,
		XSGD1ChainGenesis:          uc.XSGD1ChainGenesis,
		ETC1ChainGenesis:           uc.ETC1ChainGenesis,
		R1000ChainGenesis:          uc.R1000ChainGenesis,
		R10ChainGenesis:            uc.R10ChainGenesis,
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
