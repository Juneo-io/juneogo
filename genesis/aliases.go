// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"path"
	"strings"

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
		if uChain.ChainName == "JVM-Chain" {
			apiAliases[endpoint] = []string{
				"JVM",
				path.Join(constants.ChainAliasPrefix, "JVM"),
			}
			chainAliases[chainID] = []string{"JVM"}
		}
		if uChain.VMID == constants.EVMID && strings.Contains(uChain.ChainName, "-Chain") {
			prefix := strings.Split(uChain.ChainName, "-")[0]
			apiAliases[endpoint] = []string{
				prefix,
				path.Join(constants.ChainAliasPrefix, prefix),
			}
			chainAliases[chainID] = []string{prefix}
		}
	}
	return apiAliases, chainAliases, nil
}
