// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestNewExportTx(t *testing.T) {
	env := newEnvironment(t, banff)
	env.ctx.Lock.Lock()
	defer env.ctx.Lock.Unlock()

	type test struct {
		description        string
		destinationChainID ids.ID
		sourceKeys         []*secp256k1.PrivateKey
		timestamp          time.Time
	}

	sourceKey := preFundedKeys[0]

	tests := []test{
		{
			description:        "P->X export",
			destinationChainID: env.ctx.JVMChainID,
			sourceKeys:         []*secp256k1.PrivateKey{sourceKey},
			timestamp:          defaultValidateStartTime,
		},
		{
			description:        "P->C export",
			destinationChainID: env.ctx.JUNEChainID,
			sourceKeys:         []*secp256k1.PrivateKey{sourceKey},
			timestamp:          env.config.ApricotPhase5Time,
		},
	}

	to := ids.GenerateTestShortID()
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			require := require.New(t)

			tx, err := env.txBuilder.NewExportTx(
				tt.destinationChainID,
				[]*avax.TransferableOutput{{
					Asset: avax.Asset{ID: env.ctx.JUNEAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: defaultBalance - defaultTxFee,
						OutputOwners: secp256k1fx.OutputOwners{
							Locktime:  0,
							Threshold: 1,
							Addrs:     []ids.ShortID{to},
						},
					},
				}},
				tt.sourceKeys,
			)
			require.NoError(err)

			stateDiff, err := state.NewDiff(lastAcceptedID, env)
			require.NoError(err)

			stateDiff.SetTimestamp(tt.timestamp)

			verifier := StandardTxExecutor{
				Backend: &env.backend,
				State:   stateDiff,
				Tx:      tx,
			}
			require.NoError(tx.Unsigned.Visit(&verifier))
		})
	}
}
