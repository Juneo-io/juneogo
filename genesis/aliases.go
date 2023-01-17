// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"path"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/nftfx"
	"github.com/Juneo-io/juneogo/vms/propertyfx"
	"github.com/Juneo-io/juneogo/vms/relayvm/genesis"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

// Aliases returns the default aliases based on the network ID
func Aliases(genesisBytes []byte) (map[string][]string, map[ids.ID][]string, error) {
	apiAliases := map[string][]string{
		path.Join(constants.ChainAliasPrefix, constants.RelayChainID.String()): {
			"P",
			"platform",
			path.Join(constants.ChainAliasPrefix, "P"),
			path.Join(constants.ChainAliasPrefix, "platform"),
		},
	}
	chainAliases := map[ids.ID][]string{
		constants.RelayChainID: {"P", "platform"},
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
		case "X Chain":
			apiAliases[endpoint] = []string{
				"X",
				path.Join(constants.ChainAliasPrefix, "X"),
			}
			chainAliases[chainID] = GetAssetChainAliases()
		case "June Chain":
			apiAliases[endpoint] = []string{
				"June",
				path.Join(constants.ChainAliasPrefix, "June"),
			}
			chainAliases[chainID] = GetJuneChainAliases()
		}
	}
	return apiAliases, chainAliases, nil
}

func GetJuneChainAliases() []string {
	return []string{"June"}
}

func GetAssetChainAliases() []string {
	return []string{"X"}
}

func GetVMAliases() map[ids.ID][]string {
	return map[ids.ID][]string{
		constants.RelayVMID: {"platform"},
		constants.JVMID:     {"jvm"},
		constants.EVMID:     {"evm"},
		secp256k1fx.ID:      {"secp256k1fx"},
		nftfx.ID:            {"nftfx"},
		propertyfx.ID:       {"propertyfx"},
	}
}
