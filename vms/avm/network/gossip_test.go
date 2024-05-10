// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/vms/avm/fxs"
	"github.com/Juneo-io/juneogo/vms/avm/txs"
	"github.com/Juneo-io/juneogo/vms/avm/txs/mempool"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var _ TxVerifier = (*testVerifier)(nil)

type testVerifier struct {
	err error
}

func (v testVerifier) VerifyTx(*txs.Tx) error {
	return v.err
}

func TestMarshaller(t *testing.T) {
	require := require.New(t)

	parser, err := txs.NewParser(
		[]fxs.Fx{
			&secp256k1fx.Fx{},
		},
	)
	require.NoError(err)

	marhsaller := txParser{
		parser: parser,
	}

	want := &txs.Tx{Unsigned: &txs.BaseTx{}}
	require.NoError(want.Initialize(parser.Codec()))

	bytes, err := marhsaller.MarshalGossip(want)
	require.NoError(err)

	got, err := marhsaller.UnmarshalGossip(bytes)
	require.NoError(err)
	require.Equal(want.GossipID(), got.GossipID())
}

func TestGossipMempoolAdd(t *testing.T) {
	require := require.New(t)

	metrics := prometheus.NewRegistry()
	toEngine := make(chan common.Message, 1)

	baseMempool, err := mempool.New("", metrics, toEngine)
	require.NoError(err)

	parser, err := txs.NewParser(nil)
	require.NoError(err)

	mempool, err := newGossipMempool(
		baseMempool,
		metrics,
		logging.NoLog{},
		testVerifier{},
		parser,
		DefaultConfig.ExpectedBloomFilterElements,
		DefaultConfig.ExpectedBloomFilterFalsePositiveProbability,
		DefaultConfig.MaxBloomFilterFalsePositiveProbability,
	)
	require.NoError(err)

	tx := &txs.Tx{
		Unsigned: &txs.BaseTx{
			BaseTx: avax.BaseTx{
				Ins: []*avax.TransferableInput{},
			},
		},
		TxID: ids.GenerateTestID(),
	}

	require.NoError(mempool.Add(tx))
	require.True(mempool.bloom.Has(tx))
}

func TestGossipMempoolAddVerified(t *testing.T) {
	require := require.New(t)

	metrics := prometheus.NewRegistry()
	toEngine := make(chan common.Message, 1)

	baseMempool, err := mempool.New("", metrics, toEngine)
	require.NoError(err)

	parser, err := txs.NewParser(nil)
	require.NoError(err)

	mempool, err := newGossipMempool(
		baseMempool,
		metrics,
		logging.NoLog{},
		testVerifier{
			err: errTest, // We shouldn't be attempting to verify the tx in this flow
		},
		parser,
		DefaultConfig.ExpectedBloomFilterElements,
		DefaultConfig.ExpectedBloomFilterFalsePositiveProbability,
		DefaultConfig.MaxBloomFilterFalsePositiveProbability,
	)
	require.NoError(err)

	tx := &txs.Tx{
		Unsigned: &txs.BaseTx{
			BaseTx: avax.BaseTx{
				Ins: []*avax.TransferableInput{},
			},
		},
		TxID: ids.GenerateTestID(),
	}

	require.NoError(mempool.AddWithoutVerification(tx))
	require.True(mempool.bloom.Has(tx))
}
