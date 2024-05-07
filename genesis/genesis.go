// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/avm/fxs"
	"github.com/ava-labs/avalanchego/vms/nftfx"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/propertyfx"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	xchaintxs "github.com/ava-labs/avalanchego/vms/avm/txs"
	pchaintxs "github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

const (
	defaultEncoding    = formatting.Hex
	configChainIDAlias = "JVM"
)

var (
	errStakeDurationTooHigh            = errors.New("initial stake duration larger than maximum configured")
	errNoInitiallyStakedFunds          = errors.New("initial staked funds cannot be empty")
	errNoSupply                        = errors.New("initial supply must be > 0")
	errNoStakeDuration                 = errors.New("initial stake duration must be > 0")
	errNoStakers                       = errors.New("initial stakers must be > 0")
	errNoEVMChainGenesis               = errors.New("evm genesis cannot be empty")
	errNoTxs                           = errors.New("genesis creates no transactions")
	errNoAllocationToStake             = errors.New("no allocation to stake")
	errDuplicateInitiallyStakedAddress = errors.New("duplicate initially staked address")
	errConflictingNetworkIDs           = errors.New("conflicting networkIDs")
	errFutureStartTime                 = errors.New("startTime cannot be in the future")
	errInitialStakeDurationTooLow      = errors.New("initial stake duration is too low")
	errOverridesStandardNetworkConfig  = errors.New("overrides standard network genesis config")
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
		// ethAddrs could claim to the same avaxAddr.
		allocationSet.Add(allocation.AVAXAddr)
	}

	for _, staker := range config.InitialStakedFunds {
		if initialStakedFundsSet.Contains(staker) {
			avaxAddr, err := address.Format(
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
				"%w: %s",
				errDuplicateInitiallyStakedAddress,
				avaxAddr,
			)
		}
		initialStakedFundsSet.Add(staker)

		if !allocationSet.Contains(staker) {
			avaxAddr, err := address.Format(
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
				"%w in address %s",
				errNoAllocationToStake,
				avaxAddr,
			)
		}
	}

	return nil
}

// validateConfig returns an error if the provided
// *Config is not considered valid.
func validateConfig(networkID uint32, config *Config, stakingCfg *StakingConfig) error {
	if networkID != config.NetworkID {
		return fmt.Errorf(
			"%w: expected %d but config contains %d",
			errConflictingNetworkIDs,
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
			"%w: %s",
			errFutureStartTime,
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

	// Initial stake duration of genesis validators must be
	// not larger than maximal stake duration specified for any validator.
	if config.InitialStakeDuration > uint64(stakingCfg.MaxStakeDuration.Seconds()) {
		return errStakeDurationTooHigh
	}

	if len(config.InitialStakers) == 0 {
		return errNoStakers
	}

	offsetTimeRequired := config.InitialStakeDurationOffset * uint64(len(config.InitialStakers)-1)
	if offsetTimeRequired > config.InitialStakeDuration {
		return fmt.Errorf(
			"%w must be at least %d",
			errInitialStakeDurationTooLow,
			offsetTimeRequired,
		)
	}

	if err := validateInitialStakedFunds(config); err != nil {
		return fmt.Errorf("initial staked funds validation failed: %w", err)
	}

	if len(config.JUNEChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.USDT1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.USD1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.DAI1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.EUR1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.SGD1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.GLD1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.MBTC1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.DOGE1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.LTC1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.BCH1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}
	if len(config.LINK1ChainGenesis) == 0 {
		return errNoEVMChainGenesis
	}

	return nil
}

// FromFile returns the genesis data of the Platform Chain.
//
// Since an Avalanche network has exactly one Platform Chain, and the Platform
// Chain defines the genesis state of the network (who is staking, which chains
// exist, etc.), defining the genesis state of the Platform Chain is the same as
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
//
//  1. The byte representation of the genesis state of the platform chain
//     (ie the genesis state of the network)
//  2. The asset ID of AVAX
func FromFile(networkID uint32, filepath string, stakingCfg *StakingConfig) ([]byte, ids.ID, error) {
	switch networkID {
	case constants.MainnetID, constants.TestnetID, constants.LocalID:
		return nil, ids.ID{}, fmt.Errorf(
			"%w: %s",
			errOverridesStandardNetworkConfig,
			constants.NetworkName(networkID),
		)
	}

	config, err := GetConfigFile(filepath)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("unable to load provided genesis config at %s: %w", filepath, err)
	}

	if err := validateConfig(networkID, config, stakingCfg); err != nil {
		return nil, ids.ID{}, fmt.Errorf("genesis config validation failed: %w", err)
	}

	return FromConfig(config)
}

// FromFlag returns the genesis data of the Platform Chain.
//
// Since an Avalanche network has exactly one Platform Chain, and the Platform
// Chain defines the genesis state of the network (who is staking, which chains
// exist, etc.), defining the genesis state of the Platform Chain is the same as
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
//
//  1. The byte representation of the genesis state of the platform chain
//     (ie the genesis state of the network)
//  2. The asset ID of AVAX
func FromFlag(networkID uint32, genesisContent string, stakingCfg *StakingConfig) ([]byte, ids.ID, error) {
	switch networkID {
	case constants.MainnetID, constants.TestnetID, constants.LocalID:
		return nil, ids.ID{}, fmt.Errorf(
			"%w: %s",
			errOverridesStandardNetworkConfig,
			constants.NetworkName(networkID),
		)
	}

	customConfig, err := GetConfigContent(genesisContent)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("unable to load genesis content from flag: %w", err)
	}

	if err := validateConfig(networkID, customConfig, stakingCfg); err != nil {
		return nil, ids.ID{}, fmt.Errorf("genesis config validation failed: %w", err)
	}

	return FromConfig(customConfig)
}

// FromConfig returns:
//
//  1. The byte representation of the genesis state of the platform chain
//     (ie the genesis state of the network)
//  2. The asset ID of AVAX
func FromConfig(config *Config) ([]byte, ids.ID, error) {
	hrp := constants.GetHRP(config.NetworkID)

	amount := uint64(0)
	assetsCount := int(0)

	var june, usdt1, usd1, dai1, eur1, sgd1, gld1, mbtc1, doge1, ltc1, bch1, link1 avm.AssetDefinition

	// Specify the genesis state of the JVM
	avmArgs := avm.BuildGenesisArgs{
		NetworkID: json.Uint32(config.NetworkID),
		Encoding:  defaultEncoding,
	}
	{
		june = avm.AssetDefinition{
			Name:         "JUNE",
			Symbol:       "JUNE",
			Denomination: 9,
			InitialState: map[string][]interface{}{},
		}
		memoBytes := []byte{}
		jvmAllocations := []Allocation(nil)
		for _, allocation := range config.Allocations {
			if allocation.InitialAmount > 0 {
				jvmAllocations = append(jvmAllocations, allocation)
			}
		}
		utils.Sort(jvmAllocations)

		for _, allocation := range jvmAllocations {
			addr, err := address.FormatBech32(hrp, allocation.AVAXAddr.Bytes())
			if err != nil {
				return nil, ids.ID{}, err
			}

			june.InitialState["fixedCap"] = append(june.InitialState["fixedCap"], avm.Holder{
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

		zeroAddress, err := address.FormatBech32(
			hrp,
			make([]byte, 20),
		)
		if err != nil {
			return nil, ids.Empty, fmt.Errorf("couldn't parse zero address: %w", err)
		}
		usdt1 = createFixedAsset("Tether USDt", "USDT1", 9, zeroAddress)
		usd1 = createFixedAsset("USD1", "USD1", 9, zeroAddress)
		dai1 = createFixedAsset("Dai", "DAI1", 9, zeroAddress)
		eur1 = createFixedAsset("EUR1", "EUR1", 9, zeroAddress)
		sgd1 = createFixedAsset("SGD1", "SGD1", 9, zeroAddress)
		gld1 = createFixedAsset("Gold1", "GLD1", 9, zeroAddress)
		mbtc1 = createFixedAsset("Mili Bitcoin", "mBTC1", 9, zeroAddress)
		doge1 = createFixedAsset("Dogecoin", "DOGE1", 9, zeroAddress)
		ltc1 = createFixedAsset("Litecoin", "LTC1", 9, zeroAddress)
		bch1 = createFixedAsset("Bitcoin Cash", "BCH1", 9, zeroAddress)
		link1 = createFixedAsset("Chainlink", "LINK1", 9, zeroAddress)

		avmArgs.GenesisData = map[string]avm.AssetDefinition{
			june.Symbol:  june,
			usdt1.Symbol: usdt1,
			usd1.Symbol:  usd1,
			dai1.Symbol:  dai1,
			eur1.Symbol:  eur1,
			sgd1.Symbol:  sgd1,
			gld1.Symbol:  gld1,
			mbtc1.Symbol: mbtc1,
			doge1.Symbol: doge1,
			ltc1.Symbol:  ltc1,
			bch1.Symbol:  bch1,
			link1.Symbol: link1,
		}
		assetsCount = len(avmArgs.GenesisData)
	}
	avmReply := avm.BuildGenesisReply{}

	avmSS := avm.CreateStaticService()
	err := avmSS.BuildGenesis(nil, &avmArgs, &avmReply)
	if err != nil {
		return nil, ids.ID{}, err
	}

	bytes, err := formatting.Decode(defaultEncoding, avmReply.Bytes)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't parse avm genesis reply: %w", err)
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

	initiallyStaked := set.Of(config.InitialStakedFunds...)
	skippedAllocations := []Allocation(nil)

	// Specify the initial state of the Platform Chain
	platformvmArgs := api.BuildGenesisArgs{
		AvaxAssetID:      assetsIDs[june.Symbol],
		NetworkID:        json.Uint32(config.NetworkID),
		RewardPoolSupply: json.Uint64(config.RewardPoolSupply),
		Time:             json.Uint64(config.StartTime),
		InitialSupply:    json.Uint64(initialSupply),
		Message:          config.Message,
		Encoding:         defaultEncoding,
	}
	for _, allocation := range config.Allocations {
		if initiallyStaked.Contains(allocation.AVAXAddr) {
			skippedAllocations = append(skippedAllocations, allocation)
			continue
		}
		addr, err := address.FormatBech32(hrp, allocation.AVAXAddr.Bytes())
		if err != nil {
			return nil, ids.ID{}, err
		}
		for _, unlock := range allocation.UnlockSchedule {
			if unlock.Amount > 0 {
				msgStr, err := formatting.Encode(defaultEncoding, allocation.ETHAddr.Bytes())
				if err != nil {
					return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
				}
				platformvmArgs.UTXOs = append(platformvmArgs.UTXOs,
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
			addr, err := address.FormatBech32(hrp, allocation.AVAXAddr.Bytes())
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

		platformvmArgs.Validators = append(platformvmArgs.Validators,
			api.GenesisPermissionlessValidator{
				GenesisValidator: api.GenesisValidator{
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
				Signer:             staker.Signer,
			},
		)
	}

	// Specify the chains that exist upon this network's creation
	juneGenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.JUNEChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	usdt1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.USDT1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	usd1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.USD1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	dai1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.DAI1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	eur1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.EUR1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	sgd1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.SGD1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	gld1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.GLD1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	mbtc1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.MBTC1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	doge1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.DOGE1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	ltc1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.LTC1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	bch1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.BCH1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	link1GenesisStr, err := formatting.Encode(defaultEncoding, []byte(config.LINK1ChainGenesis))
	if err != nil {
		return nil, ids.Empty, fmt.Errorf("couldn't encode message: %w", err)
	}
	platformvmArgs.Chains = []api.Chain{
		{
			GenesisData: avmReply.Bytes,
			SubnetID:  constants.PrimaryNetworkID,
			VMID:        constants.AVMID,
			FxIDs: []ids.ID{
				secp256k1fx.ID,
				nftfx.ID,
				propertyfx.ID,
			},
			Name:         "JVM-Chain",
			ChainAssetID: assetsIDs[june.Symbol],
		},
		{
			GenesisData:  juneGenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "JUNE-Chain",
			ChainAssetID: assetsIDs[june.Symbol],
		},
		{
			GenesisData:  usdt1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "USDT1-Chain",
			ChainAssetID: assetsIDs[usdt1.Symbol],
		},
		{
			GenesisData:  usd1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "USD1-Chain",
			ChainAssetID: assetsIDs[usd1.Symbol],
		},
		{
			GenesisData:  dai1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "DAI1-Chain",
			ChainAssetID: assetsIDs[dai1.Symbol],
		},
		{
			GenesisData:  eur1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "EUR1-Chain",
			ChainAssetID: assetsIDs[eur1.Symbol],
		},
		{
			GenesisData:  sgd1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "SGD1-Chain",
			ChainAssetID: assetsIDs[sgd1.Symbol],
		},
		{
			GenesisData:  gld1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "GLD1-Chain",
			ChainAssetID: assetsIDs[gld1.Symbol],
		},
		{
			GenesisData:  mbtc1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "mBTC1-Chain",
			ChainAssetID: assetsIDs[mbtc1.Symbol],
		},
		{
			GenesisData:  doge1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "DOGE1-Chain",
			ChainAssetID: assetsIDs[doge1.Symbol],
		},
		{
			GenesisData:  ltc1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "LTC1-Chain",
			ChainAssetID: assetsIDs[ltc1.Symbol],
		},
		{
			GenesisData:  bch1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "BCH1-Chain",
			ChainAssetID: assetsIDs[bch1.Symbol],
		},
		{
			GenesisData:  link1GenesisStr,
			SubnetID:   constants.PrimaryNetworkID,
			VMID:         constants.EVMID,
			Name:         "LINK1-Chain",
			ChainAssetID: assetsIDs[link1.Symbol],
		},
	}

	platformvmReply := api.BuildGenesisReply{}
	platformvmSS := api.StaticService{}
	if err := platformvmSS.BuildGenesis(nil, &platformvmArgs, &platformvmReply); err != nil {
		return nil, ids.ID{}, fmt.Errorf("problem while building platform chain's genesis state: %w", err)
	}

	genesisBytes, err := formatting.Decode(platformvmReply.Encoding, platformvmReply.Bytes)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("problem parsing platformvm genesis bytes: %w", err)
	}

	return genesisBytes, assetsIDs[june.Symbol], nil
}

func createFixedAsset(name string, symbol string, denomination json.Uint8, address string) avm.AssetDefinition {
	asset := avm.AssetDefinition{
		Name:         name,
		Symbol:       symbol,
		Denomination: denomination,
		InitialState: map[string][]interface{}{},
	}
	asset.InitialState["fixedCap"] = append(asset.InitialState["fixedCap"], avm.Holder{
		Amount:  json.Uint64(0),
		Address: address,
	})
	return asset
}

func splitAllocations(allocations []Allocation, numSplits int) map[ids.ShortID][]Allocation {
	allNodeAllocations := make(map[ids.ShortID][]Allocation)
	for _, allocation := range allocations {
		allNodeAllocations[allocation.AVAXAddr] = append(allNodeAllocations[allocation.AVAXAddr], allocation)
	}
	return allNodeAllocations
}

func VMGenesis(genesisBytes []byte, vmID ids.ID) ([]*pchaintxs.CreateChainTx, error) {
	genesis, err := genesis.Parse(genesisBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}
	txs := []*pchaintxs.CreateChainTx{}
	for _, chain := range genesis.Chains {
		uChain := chain.Unsigned.(*pchaintxs.CreateChainTx)
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
	parser, err := xchaintxs.NewParser(
		time.Time{},
		[]fxs.Fx{
			&secp256k1fx.Fx{},
		},
	)
	if err != nil {
		return map[string]ids.ID{}, err
	}

	genesisCodec := parser.GenesisCodec()
	genesis := avm.Genesis{}
	if _, err := genesisCodec.Unmarshal(jvmGenesisBytes, &genesis); err != nil {
		return map[string]ids.ID{}, err
	}

	if len(genesis.Txs) == 0 {
		return map[string]ids.ID{}, errNoTxs
	}
	txs := map[string]ids.ID{}

	for i := 0; i < assetsCount; i++ {
		genesisTx := genesis.Txs[i]
		tx := xchaintxs.Tx{Unsigned: &genesisTx.CreateAssetTx}
		if err := tx.Initialize(genesisCodec); err != nil {
			return map[string]ids.ID{}, err
		}
		txs[genesisTx.Symbol] = tx.ID()
	}
	return txs, nil
}
