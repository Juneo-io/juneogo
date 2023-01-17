// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package stakeable

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/vms/components/june"
)

var errTest = errors.New("hi mom")

func TestLockOutVerify(t *testing.T) {
	tests := []struct {
		name             string
		locktime         uint64
		transferableOutF func(*gomock.Controller) june.TransferableOut
		expectedErr      error
	}{
		{
			name:     "happy path",
			locktime: 1,
			transferableOutF: func(ctrl *gomock.Controller) june.TransferableOut {
				o := june.NewMockTransferableOut(ctrl)
				o.EXPECT().Verify().Return(nil)
				return o
			},
			expectedErr: nil,
		},
		{
			name:     "invalid locktime",
			locktime: 0,
			transferableOutF: func(ctrl *gomock.Controller) june.TransferableOut {
				return nil
			},
			expectedErr: errInvalidLocktime,
		},
		{
			name:     "nested",
			locktime: 1,
			transferableOutF: func(ctrl *gomock.Controller) june.TransferableOut {
				return &LockOut{}
			},
			expectedErr: errNestedStakeableLocks,
		},
		{
			name:     "inner output fails verification",
			locktime: 1,
			transferableOutF: func(ctrl *gomock.Controller) june.TransferableOut {
				o := june.NewMockTransferableOut(ctrl)
				o.EXPECT().Verify().Return(errTest)
				return o
			},
			expectedErr: errTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			lockOut := &LockOut{
				Locktime:        tt.locktime,
				TransferableOut: tt.transferableOutF(ctrl),
			}
			require.Equal(tt.expectedErr, lockOut.Verify())
		})
	}
}

func TestLockInVerify(t *testing.T) {
	tests := []struct {
		name            string
		locktime        uint64
		transferableInF func(*gomock.Controller) june.TransferableIn
		expectedErr     error
	}{
		{
			name:     "happy path",
			locktime: 1,
			transferableInF: func(ctrl *gomock.Controller) june.TransferableIn {
				o := june.NewMockTransferableIn(ctrl)
				o.EXPECT().Verify().Return(nil)
				return o
			},
			expectedErr: nil,
		},
		{
			name:     "invalid locktime",
			locktime: 0,
			transferableInF: func(ctrl *gomock.Controller) june.TransferableIn {
				return nil
			},
			expectedErr: errInvalidLocktime,
		},
		{
			name:     "nested",
			locktime: 1,
			transferableInF: func(ctrl *gomock.Controller) june.TransferableIn {
				return &LockIn{}
			},
			expectedErr: errNestedStakeableLocks,
		},
		{
			name:     "inner input fails verification",
			locktime: 1,
			transferableInF: func(ctrl *gomock.Controller) june.TransferableIn {
				o := june.NewMockTransferableIn(ctrl)
				o.EXPECT().Verify().Return(errTest)
				return o
			},
			expectedErr: errTest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			lockOut := &LockIn{
				Locktime:       tt.locktime,
				TransferableIn: tt.transferableInF(ctrl),
			}
			require.Equal(tt.expectedErr, lockOut.Verify())
		})
	}
}
