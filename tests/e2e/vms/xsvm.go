// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vms

import (
	"fmt"
	"math"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/tests"
	"github.com/Juneo-io/juneogo/tests/fixture/e2e"
	"github.com/Juneo-io/juneogo/tests/fixture/tmpnet"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/example/xsvm"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/api"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/export"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/importtx"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/cmd/issue/transfer"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/genesis"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	supernetAName = "xsvm-a"
	supernetBName = "xsvm-b"
)

func XSVMSupernets(nodes ...*tmpnet.Node) []*tmpnet.Supernet {
	return []*tmpnet.Supernet{
		newXSVMSupernet(supernetAName, nodes...),
		newXSVMSupernet(supernetBName, nodes...),
	}
}

var _ = ginkgo.Describe("[XSVM]", func() {
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("should support transfers between supernets", func() {
		network := e2e.Env.GetNetwork()

		sourceSupernet := network.GetSupernet(supernetAName)
		require.NotNil(sourceSupernet)
		destinationSupernet := network.GetSupernet(supernetBName)
		require.NotNil(destinationSupernet)

		sourceChain := sourceSupernet.Chains[0]
		destinationChain := destinationSupernet.Chains[0]

		apiNode := network.Nodes[0]
		tests.Outf(" issuing transactions on %s (%s)\n", apiNode.NodeID, apiNode.URI)

		destinationKey, err := secp256k1.NewPrivateKey()
		require.NoError(err)

		ginkgo.By("checking that the funded key has sufficient funds for the export")
		sourceClient := api.NewClient(apiNode.URI, sourceChain.ChainID.String())
		initialSourcedBalance, err := sourceClient.Balance(
			e2e.DefaultContext(),
			sourceChain.PreFundedKey.Address(),
			sourceChain.ChainID,
		)
		require.NoError(err)
		require.GreaterOrEqual(initialSourcedBalance, units.Schmeckle)

		ginkgo.By(fmt.Sprintf("exporting from chain %s on supernet %s", sourceChain.ChainID, sourceSupernet.SupernetID))
		exportTxStatus, err := export.Export(
			e2e.DefaultContext(),
			&export.Config{
				URI:                apiNode.URI,
				SourceChainID:      sourceChain.ChainID,
				DestinationChainID: destinationChain.ChainID,
				Amount:             units.Schmeckle,
				To:                 destinationKey.Address(),
				PrivateKey:         sourceChain.PreFundedKey,
			},
		)
		require.NoError(err)
		tests.Outf(" issued transaction with ID: %s\n", exportTxStatus.TxID)

		ginkgo.By("checking that the export transaction has been accepted on all nodes")
		for _, node := range network.Nodes[1:] {
			require.NoError(api.WaitForAcceptance(
				e2e.DefaultContext(),
				api.NewClient(node.URI, sourceChain.ChainID.String()),
				sourceChain.PreFundedKey.Address(),
				exportTxStatus.Nonce,
			))
		}

		ginkgo.By(fmt.Sprintf("issuing transaction on chain %s on supernet %s to activate snowman++ consensus",
			destinationChain.ChainID, destinationSupernet.SupernetID))
		recipientKey, err := secp256k1.NewPrivateKey()
		require.NoError(err)
		transferTxStatus, err := transfer.Transfer(
			e2e.DefaultContext(),
			&transfer.Config{
				URI:        apiNode.URI,
				ChainID:    destinationChain.ChainID,
				AssetID:    destinationChain.ChainID,
				Amount:     units.Schmeckle,
				To:         recipientKey.Address(),
				PrivateKey: destinationChain.PreFundedKey,
			},
		)
		require.NoError(err)
		tests.Outf(" issued transaction with ID: %s\n", transferTxStatus.TxID)

		ginkgo.By(fmt.Sprintf("importing to blockchain %s on supernet %s", destinationChain.ChainID, destinationSupernet.SupernetID))
		sourceURIs := make([]string, len(network.Nodes))
		for i, node := range network.Nodes {
			sourceURIs[i] = node.URI
		}
		importTxStatus, err := importtx.Import(
			e2e.DefaultContext(),
			&importtx.Config{
				URI:                apiNode.URI,
				SourceURIs:         sourceURIs,
				SourceChainID:      sourceChain.ChainID.String(),
				DestinationChainID: destinationChain.ChainID.String(),
				TxID:               exportTxStatus.TxID,
				PrivateKey:         destinationKey,
			},
		)
		require.NoError(err)
		tests.Outf(" issued transaction with ID: %s\n", importTxStatus.TxID)

		ginkgo.By("checking that the balance of the source key has decreased")
		sourceBalance, err := sourceClient.Balance(e2e.DefaultContext(), sourceChain.PreFundedKey.Address(), sourceChain.ChainID)
		require.NoError(err)
		require.GreaterOrEqual(initialSourcedBalance-units.Schmeckle, sourceBalance)

		ginkgo.By("checking that the balance of the destination key is non-zero")
		destinationClient := api.NewClient(apiNode.URI, destinationChain.ChainID.String())
		destinationBalance, err := destinationClient.Balance(e2e.DefaultContext(), destinationKey.Address(), sourceChain.ChainID)
		require.NoError(err)
		require.Equal(units.Schmeckle, destinationBalance)
	})
})

func newXSVMSupernet(name string, nodes ...*tmpnet.Node) *tmpnet.Supernet {
	if len(nodes) == 0 {
		panic("a supernet must be validated by at least one node")
	}

	key, err := secp256k1.NewPrivateKey()
	if err != nil {
		panic(err)
	}

	genesisBytes, err := genesis.Codec.Marshal(genesis.CodecVersion, &genesis.Genesis{
		Timestamp: time.Now().Unix(),
		Allocations: []genesis.Allocation{
			{
				Address: key.Address(),
				Balance: math.MaxUint64,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	return &tmpnet.Supernet{
		Name: name,
		Chains: []*tmpnet.Chain{
			{
				VMID:         xsvm.ID,
				Genesis:      genesisBytes,
				PreFundedKey: key,
			},
		},
		ValidatorIDs: tmpnet.NodesToIDs(nodes...),
	}
}
