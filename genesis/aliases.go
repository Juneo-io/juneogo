// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"path"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/nftfx"
	"github.com/Juneo-io/juneogo/vms/platformvm/genesis"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/propertyfx"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	PChainAliases = []string{"P", "platform"}
	VMAliases     = map[ids.ID][]string{
		constants.PlatformVMID: {"platform"},
		constants.AVMID:        {"jvm"},
		constants.EVMID:        {"jevm"},
		secp256k1fx.ID:         {"secp256k1fx"},
		nftfx.ID:               {"nftfx"},
		propertyfx.ID:          {"propertyfx"},
	}
)

// Aliases returns the default aliases based on the network ID
func Aliases(genesisBytes []byte) (map[string][]string, map[ids.ID][]string, error) {
	apiAliases := map[string][]string{
		path.Join(constants.ChainAliasPrefix, constants.PlatformChainID.String()): {
			"P",
			"platform",
			path.Join(constants.ChainAliasPrefix, "P"),
			path.Join(constants.ChainAliasPrefix, "platform"),
		},
	}
	chainAliases := map[ids.ID][]string{
		constants.PlatformChainID: PChainAliases,
	}

	genesis, err := genesis.Parse(genesisBytes) // TODO let's not re-create genesis to do aliasing
	if err != nil {
		return nil, nil, err
	}
	for _, chain := range genesis.Chains {
		uChain := chain.Unsigned.(*txs.CreateChainTx)
		chainID := chain.ID()
		endpoint := path.Join(constants.ChainAliasPrefix, chainID.String())
		switch uChain.ChainName {
		case "JVM-Chain":
			apiAliases[endpoint] = []string{
				"JVM",
				path.Join(constants.ChainAliasPrefix, "JVM"),
			}
			chainAliases[chainID] = []string{"JVM"}
		case "JUNE-Chain":
			apiAliases[endpoint] = []string{
				"JUNE",
				path.Join(constants.ChainAliasPrefix, "JUNE"),
			}
			chainAliases[chainID] = []string{"JUNE"}
		case "USDT1-Chain":
			apiAliases[endpoint] = []string{
				"USDT1",
				path.Join(constants.ChainAliasPrefix, "USDT1"),
			}
			chainAliases[chainID] = []string{"USDT1"}
		case "USD1-Chain":
			apiAliases[endpoint] = []string{
				"USD1",
				path.Join(constants.ChainAliasPrefix, "USD1"),
			}
			chainAliases[chainID] = []string{"USD1"}
		case "DAI1-Chain":
			apiAliases[endpoint] = []string{
				"DAI1",
				path.Join(constants.ChainAliasPrefix, "DAI1"),
			}
			chainAliases[chainID] = []string{"DAI1"}
		case "EUR1-Chain":
			apiAliases[endpoint] = []string{
				"EUR1",
				path.Join(constants.ChainAliasPrefix, "EUR1"),
			}
			chainAliases[chainID] = []string{"EUR1"}
		case "SGD1-Chain":
			apiAliases[endpoint] = []string{
				"SGD1",
				path.Join(constants.ChainAliasPrefix, "SGD1"),
			}
			chainAliases[chainID] = []string{"SGD1"}
		case "GLD1-Chain":
			apiAliases[endpoint] = []string{
				"GLD1",
				path.Join(constants.ChainAliasPrefix, "GLD1"),
			}
			chainAliases[chainID] = []string{"GLD1"}
		case "mBTC1-Chain":
			apiAliases[endpoint] = []string{
				"mBTC1",
				path.Join(constants.ChainAliasPrefix, "mBTC1"),
			}
			chainAliases[chainID] = []string{"mBTC1"}
		case "DOGE1-Chain":
			apiAliases[endpoint] = []string{
				"DOGE1",
				path.Join(constants.ChainAliasPrefix, "DOGE1"),
			}
			chainAliases[chainID] = []string{"DOGE1"}
		case "LTC1-Chain":
			apiAliases[endpoint] = []string{
				"LTC1",
				path.Join(constants.ChainAliasPrefix, "LTC1"),
			}
			chainAliases[chainID] = []string{"LTC1"}
		case "BCH1-Chain":
			apiAliases[endpoint] = []string{
				"BCH1",
				path.Join(constants.ChainAliasPrefix, "BCH1"),
			}
			chainAliases[chainID] = []string{"BCH1"}
		case "LINK1-Chain":
			apiAliases[endpoint] = []string{
				"LINK1",
				path.Join(constants.ChainAliasPrefix, "LINK1"),
			}
			chainAliases[chainID] = []string{"LINK1"}
		}
	}
	return apiAliases, chainAliases, nil
}
