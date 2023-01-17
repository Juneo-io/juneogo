// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/components/verify"
)

func TestRemoveSupernetValidatorTxSyntacticVerify(t *testing.T) {
	type test struct {
		name      string
		txFunc    func(*gomock.Controller) *RemoveSupernetValidatorTx
		shouldErr bool
		// If [shouldErr] and [requireSpecificErr] != nil,
		// require that the error we get is [requireSpecificErr].
		requireSpecificErr error
	}

	var (
		networkID              = uint32(1337)
		chainID                = ids.GenerateTestID()
		errInvalidSupernetAuth = errors.New("invalid supernet auth")
	)

	ctx := &snow.Context{
		ChainID:   chainID,
		NetworkID: networkID,
	}

	// A BaseTx that already passed syntactic verification.
	verifiedBaseTx := BaseTx{
		SyntacticallyVerified: true,
	}
	// Sanity check.
	require.NoError(t, verifiedBaseTx.SyntacticVerify(ctx))

	// A BaseTx that passes syntactic verification.
	validBaseTx := BaseTx{
		BaseTx: june.BaseTx{
			NetworkID:    networkID,
			BlockchainID: chainID,
		},
	}
	// Sanity check.
	require.NoError(t, validBaseTx.SyntacticVerify(ctx))
	// Make sure we're not caching the verification result.
	require.False(t, validBaseTx.SyntacticallyVerified)

	// A BaseTx that fails syntactic verification.
	invalidBaseTx := BaseTx{}
	// Sanity check.
	require.Error(t, invalidBaseTx.SyntacticVerify(ctx))

	tests := []test{
		{
			name: "nil tx",
			txFunc: func(*gomock.Controller) *RemoveSupernetValidatorTx {
				return nil
			},
			shouldErr: true,
		},
		{
			name: "already verified",
			txFunc: func(*gomock.Controller) *RemoveSupernetValidatorTx {
				return &RemoveSupernetValidatorTx{BaseTx: verifiedBaseTx}
			},
			shouldErr: false,
		},
		{
			name: "invalid BaseTx",
			txFunc: func(*gomock.Controller) *RemoveSupernetValidatorTx {
				return &RemoveSupernetValidatorTx{
					// Set supernetID so we don't error on that check.
					Supernet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID: ids.GenerateTestNodeID(),
					BaseTx: invalidBaseTx,
				}
			},
			shouldErr: true,
		},
		{
			name: "invalid supernetID",
			txFunc: func(*gomock.Controller) *RemoveSupernetValidatorTx {
				return &RemoveSupernetValidatorTx{
					BaseTx: validBaseTx,
					// Set NodeID so we don't error on that check.
					NodeID:   ids.GenerateTestNodeID(),
					Supernet: constants.PrimaryNetworkID,
				}
			},
			shouldErr:          true,
			requireSpecificErr: errRemovePrimaryNetworkValidator,
		},
		{
			name: "invalid supernetAuth",
			txFunc: func(ctrl *gomock.Controller) *RemoveSupernetValidatorTx {
				// This SupernetAuth fails verification.
				invalidSupernetAuth := verify.NewMockVerifiable(ctrl)
				invalidSupernetAuth.EXPECT().Verify().Return(errInvalidSupernetAuth)
				return &RemoveSupernetValidatorTx{
					// Set supernetID so we don't error on that check.
					Supernet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID:       ids.GenerateTestNodeID(),
					BaseTx:       validBaseTx,
					SupernetAuth: invalidSupernetAuth,
				}
			},
			shouldErr:          true,
			requireSpecificErr: errInvalidSupernetAuth,
		},
		{
			name: "passes verification",
			txFunc: func(ctrl *gomock.Controller) *RemoveSupernetValidatorTx {
				// This SupernetAuth passes verification.
				validSupernetAuth := verify.NewMockVerifiable(ctrl)
				validSupernetAuth.EXPECT().Verify().Return(nil)
				return &RemoveSupernetValidatorTx{
					// Set supernetID so we don't error on that check.
					Supernet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID:       ids.GenerateTestNodeID(),
					BaseTx:       validBaseTx,
					SupernetAuth: validSupernetAuth,
				}
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tx := tt.txFunc(ctrl)
			err := tx.SyntacticVerify(ctx)
			if tt.shouldErr {
				require.Error(err)
				if tt.requireSpecificErr != nil {
					require.ErrorIs(err, tt.requireSpecificErr)
				}
				return
			}
			require.NoError(err)
			require.True(tx.SyntacticallyVerified)
		})
	}
}
