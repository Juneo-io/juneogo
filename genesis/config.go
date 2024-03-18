// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"cmp"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

var (
	_ utils.Sortable[Allocation] = Allocation{}

	errInvalidGenesisJSON = errors.New("could not unmarshal genesis JSON")
)

type LockedAmount struct {
	Amount   uint64 `json:"amount"`
	Locktime uint64 `json:"locktime"`
}

type Allocation struct {
	ETHAddr        ids.ShortID    `json:"ethAddr"`
	AVAXAddr       ids.ShortID    `json:"avaxAddr"`
	InitialAmount  uint64         `json:"initialAmount"`
	UnlockSchedule []LockedAmount `json:"unlockSchedule"`
}

func (a Allocation) Unparse(networkID uint32) (UnparsedAllocation, error) {
	ua := UnparsedAllocation{
		InitialAmount:  a.InitialAmount,
		UnlockSchedule: a.UnlockSchedule,
		ETHAddr:        "0x" + hex.EncodeToString(a.ETHAddr.Bytes()),
	}
	avaxAddr, err := address.Format(
		"JVM",
		constants.GetHRP(networkID),
		a.AVAXAddr.Bytes(),
	)
	ua.AVAXAddr = avaxAddr
	return ua, err
}

func (a Allocation) Compare(other Allocation) int {
	if amountCmp := cmp.Compare(a.InitialAmount, other.InitialAmount); amountCmp != 0 {
		return amountCmp
	}
	return a.AVAXAddr.Compare(other.AVAXAddr)
}

type Staker struct {
	NodeID        ids.NodeID                `json:"nodeID"`
	RewardAddress ids.ShortID               `json:"rewardAddress"`
	DelegationFee uint32                    `json:"delegationFee"`
	Signer        *signer.ProofOfPossession `json:"signer,omitempty"`
}

func (s Staker) Unparse(networkID uint32) (UnparsedStaker, error) {
	avaxAddr, err := address.Format(
		"JVM",
		constants.GetHRP(networkID),
		s.RewardAddress.Bytes(),
	)
	return UnparsedStaker{
		NodeID:        s.NodeID,
		RewardAddress: avaxAddr,
		DelegationFee: s.DelegationFee,
		Signer:        s.Signer,
	}, err
}

// Config contains the genesis addresses used to construct a genesis
type Config struct {
	NetworkID        uint32 `json:"networkID"`
	RewardPoolSupply uint64 `json:"rewardPoolSupply"`

	Allocations []Allocation `json:"allocations"`

	StartTime                  uint64        `json:"startTime"`
	InitialStakeDuration       uint64        `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64        `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []ids.ShortID `json:"initialStakedFunds"`
	InitialStakers             []Staker      `json:"initialStakers"`

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

func (c Config) Unparse() (UnparsedConfig, error) {
	uc := UnparsedConfig{
		NetworkID:                  c.NetworkID,
		RewardPoolSupply:           c.RewardPoolSupply,
		Allocations:                make([]UnparsedAllocation, len(c.Allocations)),
		StartTime:                  c.StartTime,
		InitialStakeDuration:       c.InitialStakeDuration,
		InitialStakeDurationOffset: c.InitialStakeDurationOffset,
		InitialStakedFunds:         make([]string, len(c.InitialStakedFunds)),
		InitialStakers:             make([]UnparsedStaker, len(c.InitialStakers)),
		JUNEChainGenesis:           c.JUNEChainGenesis,
		USDT1ChainGenesis:          c.USDT1ChainGenesis,
		USD1ChainGenesis:           c.USD1ChainGenesis,
		DAI1ChainGenesis:           c.DAI1ChainGenesis,
		EUR1ChainGenesis:           c.EUR1ChainGenesis,
		SGD1ChainGenesis:           c.SGD1ChainGenesis,
		GLD1ChainGenesis:           c.GLD1ChainGenesis,
		MBTC1ChainGenesis:          c.MBTC1ChainGenesis,
		DOGE1ChainGenesis:          c.DOGE1ChainGenesis,
		LTC1ChainGenesis:           c.LTC1ChainGenesis,
		Message:                    c.Message,
	}
	for i, a := range c.Allocations {
		ua, err := a.Unparse(uc.NetworkID)
		if err != nil {
			return uc, err
		}
		uc.Allocations[i] = ua
	}
	for i, isa := range c.InitialStakedFunds {
		avaxAddr, err := address.Format(
			"JVM",
			constants.GetHRP(uc.NetworkID),
			isa.Bytes(),
		)
		if err != nil {
			return uc, err
		}
		uc.InitialStakedFunds[i] = avaxAddr
	}
	for i, is := range c.InitialStakers {
		uis, err := is.Unparse(c.NetworkID)
		if err != nil {
			return uc, err
		}
		uc.InitialStakers[i] = uis
	}

	return uc, nil
}

func (c *Config) InitialSupply() (uint64, error) {
	initialSupply := uint64(0)
	for _, allocation := range c.Allocations {
		newInitialSupply, err := math.Add64(initialSupply, allocation.InitialAmount)
		if err != nil {
			return 0, err
		}
		for _, unlock := range allocation.UnlockSchedule {
			newInitialSupply, err = math.Add64(newInitialSupply, unlock.Amount)
			if err != nil {
				return 0, err
			}
		}
		initialSupply = newInitialSupply
	}
	newInitialSupply, err := math.Add64(initialSupply, c.RewardPoolSupply)
	if err != nil {
		return 0, err
	}
	initialSupply = newInitialSupply
	return initialSupply, nil
}

var (
	// MainnetConfig is the config that should be used to generate the mainnet
	// genesis.
	MainnetConfig Config

	// SocotraConfig is the config that should be used to generate the socotra
	// genesis.
	SocotraConfig Config

	// LocalConfig is the config that should be used to generate a local
	// genesis.
	LocalConfig Config
)

func init() {
	unparsedMainnetConfig := UnparsedConfig{}
	unparsedSocotraConfig := UnparsedConfig{}
	unparsedLocalConfig := UnparsedConfig{}

	err := utils.Err(
		json.Unmarshal(mainnetGenesisConfigJSON, &unparsedMainnetConfig),
		json.Unmarshal(socotraGenesisConfigJSON, &unparsedSocotraConfig),
		json.Unmarshal(localGenesisConfigJSON, &unparsedLocalConfig),
	)
	if err != nil {
		panic(err)
	}

	MainnetConfig, err = unparsedMainnetConfig.Parse()
	if err != nil {
		panic(err)
	}

	SocotraConfig, err = unparsedSocotraConfig.Parse()
	if err != nil {
		panic(err)
	}

	LocalConfig, err = unparsedLocalConfig.Parse()
	if err != nil {
		panic(err)
	}
}

func GetConfig(networkID uint32) *Config {
	switch networkID {
	case constants.MainnetID:
		return &MainnetConfig
	case constants.TestnetID:
		return &SocotraConfig
	case constants.LocalID:
		return &LocalConfig
	default:
		tempConfig := LocalConfig
		tempConfig.NetworkID = networkID
		return &tempConfig
	}
}

// GetConfigFile loads a *Config from a provided filepath.
func GetConfigFile(fp string) (*Config, error) {
	bytes, err := os.ReadFile(filepath.Clean(fp))
	if err != nil {
		return nil, fmt.Errorf("unable to load file %s: %w", fp, err)
	}
	return parseGenesisJSONBytesToConfig(bytes)
}

// GetConfigContent loads a *Config from a provided environment variable
func GetConfigContent(genesisContent string) (*Config, error) {
	bytes, err := base64.StdEncoding.DecodeString(genesisContent)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64 content: %w", err)
	}
	return parseGenesisJSONBytesToConfig(bytes)
}

func parseGenesisJSONBytesToConfig(bytes []byte) (*Config, error) {
	var unparsedConfig UnparsedConfig
	if err := json.Unmarshal(bytes, &unparsedConfig); err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidGenesisJSON, err)
	}

	config, err := unparsedConfig.Parse()
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	return &config, nil
}
