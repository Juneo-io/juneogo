// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
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
		constants.PlatformChainID: {"P", "platform"},
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
		case "ETH1-Chain":
			apiAliases[endpoint] = []string{
				"ETH1",
				path.Join(constants.ChainAliasPrefix, "ETH1"),
			}
			chainAliases[chainID] = []string{"ETH1"}
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
		case "TUSD1-Chain":
			apiAliases[endpoint] = []string{
				"TUSD1",
				path.Join(constants.ChainAliasPrefix, "TUSD1"),
			}
			chainAliases[chainID] = []string{"TUSD1"}
		case "USDT1-Chain":
			apiAliases[endpoint] = []string{
				"USDT1",
				path.Join(constants.ChainAliasPrefix, "USDT1"),
			}
			chainAliases[chainID] = []string{"USDT1"}
		case "DAI1-Chain":
			apiAliases[endpoint] = []string{
				"DAI1",
				path.Join(constants.ChainAliasPrefix, "DAI1"),
			}
			chainAliases[chainID] = []string{"DAI1"}
		case "EUROC1-Chain":
			apiAliases[endpoint] = []string{
				"EUROC1",
				path.Join(constants.ChainAliasPrefix, "EUROC1"),
			}
			chainAliases[chainID] = []string{"EUROC1"}
		case "LTC1-Chain":
			apiAliases[endpoint] = []string{
				"LTC1",
				path.Join(constants.ChainAliasPrefix, "LTC1"),
			}
			chainAliases[chainID] = []string{"LTC1"}
		case "XLM1-Chain":
			apiAliases[endpoint] = []string{
				"XLM1",
				path.Join(constants.ChainAliasPrefix, "XLM1"),
			}
			chainAliases[chainID] = []string{"XLM1"}
		case "BCH1-Chain":
			apiAliases[endpoint] = []string{
				"BCH1",
				path.Join(constants.ChainAliasPrefix, "BCH1"),
			}
			chainAliases[chainID] = []string{"BCH1"}
		case "PAXG1-Chain":
			apiAliases[endpoint] = []string{
				"PAXG1",
				path.Join(constants.ChainAliasPrefix, "PAXG1"),
			}
			chainAliases[chainID] = []string{"PAXG1"}
		case "ICP1-Chain":
			apiAliases[endpoint] = []string{
				"ICP1",
				path.Join(constants.ChainAliasPrefix, "ICP1"),
			}
			chainAliases[chainID] = []string{"ICP1"}
		case "XIDR1-Chain":
			apiAliases[endpoint] = []string{
				"XIDR1",
				path.Join(constants.ChainAliasPrefix, "XIDR1"),
			}
			chainAliases[chainID] = []string{"XIDR1"}
		case "XSGD1-Chain":
			apiAliases[endpoint] = []string{
				"XSGD1",
				path.Join(constants.ChainAliasPrefix, "XSGD1"),
			}
			chainAliases[chainID] = []string{"XSGD1"}
		case "ETC1-Chain":
			apiAliases[endpoint] = []string{
				"ETC1",
				path.Join(constants.ChainAliasPrefix, "ETC1"),
			}
			chainAliases[chainID] = []string{"ETC1"}
		case "R1000-Chain":
			apiAliases[endpoint] = []string{
				"R1000",
				path.Join(constants.ChainAliasPrefix, "R1000"),
			}
			chainAliases[chainID] = []string{"R1000"}
		case "R10-Chain":
			apiAliases[endpoint] = []string{
				"R10",
				path.Join(constants.ChainAliasPrefix, "R10"),
			}
			chainAliases[chainID] = []string{"R10"}
		}
	}
	return apiAliases, chainAliases, nil
}

func GetVMAliases() map[ids.ID][]string {
	return map[ids.ID][]string{
		constants.PlatformVMID: {"platform"},
		constants.AVMID:        {"jvm"},
		constants.EVMID:        {"jevm"},
		secp256k1fx.ID:         {"secp256k1fx"},
		nftfx.ID:               {"nftfx"},
		propertyfx.ID:          {"propertyfx"},
	}
}
