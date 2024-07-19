// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snowtest

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/api/metrics"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/logging"
)

var (
	JVMChainID    = ids.GenerateTestID()
	JUNEChainID    = ids.GenerateTestID()
	PChainID    = constants.PlatformChainID
	JUNEAssetID = ids.GenerateTestID()

	errMissing = errors.New("missing")

	_ snow.Acceptor = noOpAcceptor{}
)

type noOpAcceptor struct{}

func (noOpAcceptor) Accept(*snow.ConsensusContext, ids.ID, []byte) error {
	return nil
}

func ConsensusContext(ctx *snow.Context) *snow.ConsensusContext {
	return &snow.ConsensusContext{
		Context:             ctx,
		Registerer:          prometheus.NewRegistry(),
		AvalancheRegisterer: prometheus.NewRegistry(),
		BlockAcceptor:       noOpAcceptor{},
		TxAcceptor:          noOpAcceptor{},
		VertexAcceptor:      noOpAcceptor{},
	}
}

func Context(tb testing.TB, chainID ids.ID) *snow.Context {
	require := require.New(tb)

	secretKey, err := bls.NewSecretKey()
	require.NoError(err)
	publicKey := bls.PublicFromSecretKey(secretKey)

	aliaser := ids.NewAliaser()
	require.NoError(aliaser.Alias(constants.PlatformChainID, "P"))
	require.NoError(aliaser.Alias(constants.PlatformChainID, constants.PlatformChainID.String()))
	require.NoError(aliaser.Alias(JVMChainID, "X"))
	require.NoError(aliaser.Alias(JVMChainID, JVMChainID.String()))
	require.NoError(aliaser.Alias(JUNEChainID, "C"))
	require.NoError(aliaser.Alias(JUNEChainID, JUNEChainID.String()))

	validatorState := &validators.TestState{
		GetSupernetIDF: func(_ context.Context, chainID ids.ID) (ids.ID, error) {
			supernetID, ok := map[ids.ID]ids.ID{
				constants.PlatformChainID: constants.PrimaryNetworkID,
				JVMChainID:                  constants.PrimaryNetworkID,
				JUNEChainID:                  constants.PrimaryNetworkID,
			}[chainID]
			if !ok {
				return ids.Empty, errMissing
			}
			return supernetID, nil
		},
	}

	return &snow.Context{
		NetworkID: constants.UnitTestID,
		SupernetID:  constants.PrimaryNetworkID,
		ChainID:   chainID,
		NodeID:    ids.EmptyNodeID,
		PublicKey: publicKey,

		JVMChainID:    JVMChainID,
		JUNEChainID:    JUNEChainID,
		JUNEAssetID: JUNEAssetID,

		Log:      logging.NoLog{},
		BCLookup: aliaser,
		Metrics:  metrics.NewOptionalGatherer(),

		ValidatorState: validatorState,
		ChainDataDir:   "",
	}
}
