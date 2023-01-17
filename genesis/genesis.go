// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"errors"
	"fmt"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/jvm"
	"github.com/Juneo-io/juneogo/vms/jvm/fxs"
	"github.com/Juneo-io/juneogo/vms/nftfx"
	"github.com/Juneo-io/juneogo/vms/propertyfx"
	"github.com/Juneo-io/juneogo/vms/relayvm/api"
	"github.com/Juneo-io/juneogo/vms/relayvm/genesis"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"

	assetchaintxs "github.com/Juneo-io/juneogo/vms/jvm/txs"
	relaychaintxs "github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

const (
	defaultEncoding    = formatting.Hex
	configChainIDAlias = "X"
)

var (
	errNoInitiallyStakedFunds = errors.New("initial staked funds cannot be empty")
	errNoSupply               = errors.New("initial supply must be > 0")
	errNoStakeDuration        = errors.New("initial stake duration must be > 0")
	errNoStakers              = errors.New("initial stakers must be > 0")
	errNoJuneChainGenesis     = errors.New("June-Chain genesis cannot be empty")
	errNoTxs                  = errors.New("genesis creates no transactions")
)

// validateInitialStakedFunds ensures all staked
// funds have allocations and that all staked
// funds are unique.
//
// This function assumes that NetworkID in *Config has already
// been checked for correctness.
func validateInitialStakedFunds(config *Config) error {
	if len(config.InitialStakedFunds) == 0 {
		return errNoInitiallyStakedFunds
	}

	allocationSet := set.Set[ids.ShortID]{}
	initialStakedFundsSet := set.Set[ids.ShortID]{}
	for _, allocation := range config.Allocations {
		// It is ok to have duplicates as different
		// ethAddrs could claim to the same juneAddr.
		allocationSet.Add(allocation.JuneAddr)
	}

	for _, staker := range config.InitialStakedFunds {
		if initialStakedFundsSet.Contains(staker) {
			juneAddr, err := address.Format(
				configChainIDAlias,
				constants.GetHRP(config.NetworkID),
				staker.Bytes(),
			)
			if err != nil {
				return fmt.Errorf(
					"unable to format address from %s",
					staker.String(),
				)
			}

			return fmt.Errorf(
				"address %s is duplicated in initial staked funds",
				juneAddr,
			)
		}
		initialStakedFundsSet.Add(staker)

		if !allocationSet.Contains(staker) {
			juneAddr, err := address.Format(
				configChainIDAlias,
				constants.GetHRP(config.NetworkID),
				staker.Bytes(),
			)
			if err != nil {
				return fmt.Errorf(
					"unable to format address from %s",
					staker.String(),
				)
			}

			return fmt.Errorf(
				"address %s does not have an allocation to stake",
				juneAddr,
			)
		}
	}

	return nil
}

// validateConfig returns an error if the provided
// *Config is not considered valid.
func validateConfig(networkID uint32, config *Config) error {
	if networkID != config.NetworkID {
		return fmt.Errorf(
			"networkID %d specified but genesis config contains networkID %d",
			networkID,
			config.NetworkID,
		)
	}

	initialSupply, err := config.InitialSupply()
	switch {
	case err != nil:
		return fmt.Errorf("unable to calculate initial supply: %w", err)
	case initialSupply == 0:
		return errNoSupply
	}

	startTime := time.Unix(int64(config.StartTime), 0)
	if time.Since(startTime) < 0 {
		return fmt.Errorf(
			"start time cannot be in the future: %s",
			startTime,
		)
	}

	// We don't impose any restrictions on the minimum
	// stake duration to enable complex testing configurations
	// but recommend setting a minimum duration of at least
	// 15 minutes.
	if config.InitialStakeDuration == 0 {
		return errNoStakeDuration
	}

	if len(config.InitialStakers) == 0 {
		return errNoStakers
	}

	offsetTimeRequired := config.InitialStakeDurationOffset * uint64(len(config.InitialStakers)-1)
	if offsetTimeRequired > config.InitialStakeDuration {
		return fmt.Errorf(
			"initial stake duration is %d but need at least %d with offset of %d",
			config.InitialStakeDuration,
			offsetTimeRequired,
			config.InitialStakeDurationOffset,
		)
	}

	if err := validateInitialStakedFunds(config); err != nil {
		return fmt.Errorf("initial staked funds validation failed: %w", err)
	}

	if len(config.JuneChainGenesis) == 0 {
		return errNoJuneChainGenesis
	}

	return nil
}

// FromFile returns the genesis data of the Relay Chain.
//
// Since an Avalanche network has exactly one Relay Chain, and the Relay
// Chain defines the genesis state of the network (who is staking, which chains
// exist, etc.), defining the genesis state of the Relay Chain is the same as
// defining the genesis state of the network.
//
// FromFile accepts:
// 1) The ID of the new network. [networkID]
// 2) The location of a custom genesis config to load. [filepath]
//
// If [filepath] is empty or the given network ID is Mainnet, Testnet, or Local, returns error.
// If [filepath] is non-empty and networkID isn't Mainnet, Testnet, or Local,
// loads the network genesis data from the config at [filepath].
//
// FromFile returns:
//  1. The byte representation of the genesis state of the relay chain
//     (ie the genesis state of the network)
//  2. The asset ID of June
func FromFile(networkID uint32, filepath string) ([]byte, ids.ID, error) {
	switch networkID {
	case constants.MainnetID, constants.TestnetID, constants.LocalID:
		return nil, ids.ID{}, fmt.Errorf(
			"cannot override genesis config for standard network %s (%d)",
			constants.NetworkName(networkID),
			networkID,
		)
	}

	config, err := GetConfigFile(filepath)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("unable to load provided genesis config at %s: %w", filepath, err)
	}

	if err := validateConfig(networkID, config); err != nil {
		return nil, ids.ID{}, fmt.Errorf("genesis config validation failed: %w", err)
	}

	return FromConfig(config)
}

// FromFlag returns the genesis data of the Relay Chain.
//
// Since an Avalanche network has exactly one Relay Chain, and the Relay
// Chain defines the genesis state of the network (who is staking, which chains
// exist, etc.), defining the genesis state of the Relay Chain is the same as
// defining the genesis state of the network.
//
// FromFlag accepts:
// 1) The ID of the new network. [networkID]
// 2) The content of a custom genesis config to load. [genesisContent]
//
// If [genesisContent] is empty or the given network ID is Mainnet, Testnet, or Local, returns error.
// If [genesisContent] is non-empty and networkID isn't Mainnet, Testnet, or Local,
// loads the network genesis data from [genesisContent].
//
// FromFlag returns:
//  1. The byte representation of the genesis state of the relay chain
//     (ie the genesis state of the network)
//  2. The asset ID of June
func FromFlag(networkID uint32, genesisContent string) ([]byte, ids.ID, error) {
	switch networkID {
	case constants.MainnetID, constants.TestnetID, constants.LocalID:
		return nil, ids.ID{}, fmt.Errorf(
			"cannot override genesis config for standard network %s (%d)",
			constants.NetworkName(networkID),
			networkID,
		)
	}

	customConfig, err := GetConfigContent(genesisContent)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("unable to load genesis content from flag: %w", err)
	}

	if err := validateConfig(networkID, customConfig); err != nil {
		return nil, ids.ID{}, fmt.Errorf("genesis config validation failed: %w", err)
	}

	return FromConfig(customConfig)
}

// FromConfig returns:
//  1. The byte representation of the genesis state of the relay chain
//     (ie the genesis state of the network)
//  2. The asset ID of June
func FromConfig(config *Config) ([]byte, ids.ID, error) {
	hrp := constants.GetHRP(config.NetworkID)

	amount := uint64(0)
	assetsCount := int(0)

	// Specify the genesis state of the JVM
	jvmArgs := jvm.BuildGenesisArgs{
		NetworkID: json.Uint32(config.NetworkID),
		Encoding:  defaultEncoding,
	}
	{
		june := jvm.AssetDefinition{
			Name:         "June",
			Symbol:       "JUNE",
			Denomination: 9,
			InitialState: map[string][]interface{}{},
		}
		memoBytes := []byte{}
		assetChainAllocations := []Allocation(nil)
		for _, allocation := range config.Allocations {
			if allocation.InitialAmount > 0 {
				assetChainAllocations = append(assetChainAllocations, allocation)
			}
		}
		utils.Sort(assetChainAllocations)

		for _, allocation := range assetChainAllocations {
			addr, err := address.FormatBech32(hrp, allocation.JuneAddr.Bytes())
			if err != nil {
				return nil, ids.ID{}, err
			}

			june.InitialState["fixedCap"] = append(june.InitialState["fixedCap"], jvm.Holder{
				Amount:  json.Uint64(allocation.InitialAmount),
				Address: addr,
			})
			memoBytes = append(memoBytes, allocation.ETHAddr.Bytes()...)
			amount += allocation.InitialAmount
		}

		var err error
		june.Memo, err = formatting.Encode(defaultEncoding, memoBytes)
		if err != nil {
			return nil, ids.Empty, fmt.Errorf("couldn't parse memo bytes to string: %w", err)
		}
		jvmArgs.GenesisData = map[string]jvm.AssetDefinition{
			"JUNE": june,
		}
		assetsCount = len(jvmArgs.GenesisData)
	}
	jvmReply := jvm.BuildGenesisReply{}

	jvmSS := jvm.CreateStaticService()
	err := jvmSS.BuildGenesis(nil, &jvmArgs, &jvmReply)
	if err != nil {
		return nil, ids.ID{}, err
	}

	bytes, err := formatting.Decode(defaultEncoding, jvmReply.Bytes)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't parse jvm genesis reply: %w", err)
	}
	assetsIDs, err := GenesisAssetsIDs(bytes, assetsCount)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't generate genesis assets IDs: %w", err)
	}

	genesisTime := time.Unix(int64(config.StartTime), 0)
	initialSupply, err := config.InitialSupply()
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't calculate the initial supply: %w", err)
	}

	initiallyStaked := set.Set[ids.ShortID]{}
	initiallyStaked.Add(config.InitialStakedFunds...)
	skippedAllocations := []Allocation(nil)

	// Specify the initial state of the Relay Chain
	relayvmArgs := api.BuildGenesisArgs{
		JuneAssetID:       assetsIDs["JUNE"],
		NetworkID:         json.Uint32(config.NetworkID),
		Time:              json.Uint64(config.StartTime),
		InitialSupply:     json.Uint64(initialSupply),
		RewardsPoolSupply: json.Uint64(config.RewardsPoolSupply),
		Message:           config.Message,
		Encoding:          defaultEncoding,
	}
	for _, allocation := range config.Allocations {
		if initiallyStaked.Contains(allocation.JuneAddr) {
			skippedAllocations = append(skippedAllocations, allocation)
			continue
		}
		addr, err := address.FormatBech32(hrp, allocation.JuneAddr.Bytes())
		if err != nil {
			return nil, ids.ID{}, err
		}
		for _, unlock := range allocation.UnlockSchedule {
			if unlock.Amount > 0 {
				msgStr, err := formatting.Encode(defaultEncoding, allocation.ETHAddr.Bytes())
				if err != nil {
					return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
				}
				relayvmArgs.UTXOs = append(relayvmArgs.UTXOs,
					api.UTXO{
						Locktime: json.Uint64(unlock.Locktime),
						Amount:   json.Uint64(unlock.Amount),
						Address:  addr,
						Message:  msgStr,
					},
				)
				amount += unlock.Amount
			}
		}
	}

	allNodeAllocations := splitAllocations(skippedAllocations, len(config.InitialStakers))
	endStakingTime := genesisTime.Add(time.Duration(config.InitialStakeDuration) * time.Second)
	stakingOffset := time.Duration(0)
	for _, staker := range config.InitialStakers {
		nodeAllocations := allNodeAllocations[staker.RewardAddress]
		endStakingTime := endStakingTime.Add(-stakingOffset)
		stakingOffset += time.Duration(config.InitialStakeDurationOffset) * time.Second

		destAddrStr, err := address.FormatBech32(hrp, staker.RewardAddress.Bytes())
		if err != nil {
			return nil, ids.ID{}, err
		}

		utxos := []api.UTXO(nil)
		for _, allocation := range nodeAllocations {
			addr, err := address.FormatBech32(hrp, allocation.JuneAddr.Bytes())
			if err != nil {
				return nil, ids.ID{}, err
			}
			for _, unlock := range allocation.UnlockSchedule {
				msgStr, err := formatting.Encode(defaultEncoding, allocation.ETHAddr.Bytes())
				if err != nil {
					return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
				}
				utxos = append(utxos, api.UTXO{
					Locktime: json.Uint64(unlock.Locktime),
					Amount:   json.Uint64(unlock.Amount),
					Address:  addr,
					Message:  msgStr,
				})
				amount += unlock.Amount
			}
		}

		delegationFee := json.Uint32(staker.DelegationFee)

		relayvmArgs.Validators = append(relayvmArgs.Validators,
			api.PermissionlessValidator{
				Staker: api.Staker{
					StartTime: json.Uint64(genesisTime.Unix()),
					EndTime:   json.Uint64(endStakingTime.Unix()),
					NodeID:    staker.NodeID,
				},
				RewardOwner: &api.Owner{
					Threshold: 1,
					Addresses: []string{destAddrStr},
				},
				Staked:             utxos,
				ExactDelegationFee: &delegationFee,
			},
		)
	}

	// Specify the chains that exist upon this network's creation
	genesisStr, err := formatting.Encode(defaultEncoding, []byte(config.JuneChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	relayvmArgs.Chains = []api.Chain{
		{
			GenesisData: jvmReply.Bytes,
			SupernetID:  constants.PrimaryNetworkID,
			VMID:        constants.JVMID,
			FxIDs: []ids.ID{
				secp256k1fx.ID,
				nftfx.ID,
				propertyfx.ID,
			},
			Name:         "X Chain",
			ChainAssetID: assetsIDs["JUNE"],
		},
		{
			GenesisData:  genesisStr,
			SupernetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "June Chain",
			ChainAssetID: assetsIDs["JUNE"],
		},
	}

	relayvmReply := api.BuildGenesisReply{}
	relayvmSS := api.StaticService{}
	if err := relayvmSS.BuildGenesis(nil, &relayvmArgs, &relayvmReply); err != nil {
		return nil, ids.ID{}, fmt.Errorf("problem while building relay chain's genesis state: %w", err)
	}

	genesisBytes, err := formatting.Decode(relayvmReply.Encoding, relayvmReply.Bytes)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("problem parsing relayvm genesis bytes: %w", err)
	}

	return genesisBytes, assetsIDs["JUNE"], nil
}

func splitAllocations(allocations []Allocation, numSplits int) map[ids.ShortID][]Allocation {
	allNodeAllocations := make(map[ids.ShortID][]Allocation)
	for _, allocation := range allocations {
		allNodeAllocations[allocation.JuneAddr] = append(allNodeAllocations[allocation.JuneAddr], allocation)
	}
	return allNodeAllocations
}

func VMGenesis(genesisBytes []byte, vmID ids.ID) ([]*relaychaintxs.CreateChainTx, error) {
	genesis, err := genesis.Parse(genesisBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}
	txs := []*relaychaintxs.CreateChainTx{}
	for _, chain := range genesis.Chains {
		uChain := chain.Unsigned.(*relaychaintxs.CreateChainTx)
		uChain.BlockchainID = chain.ID()
		if uChain.VMID == vmID {
			txs = append(txs, uChain)
		}
	}
	if len(txs) > 0 {
		return txs, nil
	} else {
		return nil, fmt.Errorf("couldn't find blockchain with VM ID %s", vmID)
	}
}

func GenesisAssetsIDs(jvmGenesisBytes []byte, assetsCount int) (map[string]ids.ID, error) {
	parser, err := assetchaintxs.NewParser([]fxs.Fx{
		&secp256k1fx.Fx{},
	})
	if err != nil {
		return map[string]ids.ID{}, err
	}

	genesisCodec := parser.GenesisCodec()
	genesis := jvm.Genesis{}
	if _, err := genesisCodec.Unmarshal(jvmGenesisBytes, &genesis); err != nil {
		return map[string]ids.ID{}, err
	}

	if len(genesis.Txs) == 0 {
		return map[string]ids.ID{}, errNoTxs
	}
	txs := map[string]ids.ID{}

	for i := 0; i < assetsCount; i++ {
		genesisTx := genesis.Txs[i]
		tx := assetchaintxs.Tx{Unsigned: &genesisTx.CreateAssetTx}
		if err := parser.InitializeGenesisTx(&tx); err != nil {
			return map[string]ids.ID{}, err
		}
		txs[genesisTx.Symbol] = tx.ID()
	}
	return txs, nil
}
