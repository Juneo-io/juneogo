// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPriorityIsCurrent(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsCurrent())
		})
	}
}

func TestPriorityIsPending(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsPending())
		})
	}
}

func TestPriorityIsValidator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsValidator())
		})
	}
}

func TestPriorityIsPermissionedValidator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsPermissionedValidator())
		})
	}
}

func TestPriorityIsDelegator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsDelegator())
		})
	}
}

func TestPriorityIsCurrentValidator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsCurrentValidator())
		})
	}
}

func TestPriorityIsCurrentDelegator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsCurrentDelegator())
		})
	}
}

func TestPriorityIsPendingValidator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsPendingValidator())
		})
	}
}

func TestPriorityIsPendingDelegator(t *testing.T) {
	tests := []struct {
		priority Priority
		expected bool
	}{
		{
			priority: PrimaryNetworkDelegatorApricotPendingPriority,
			expected: true,
		},
		{
			priority: PrimaryNetworkValidatorPendingPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorBanffPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionlessValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorPendingPriority,
			expected: true,
		},
		{
			priority: SupernetPermissionedValidatorPendingPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionedValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: SupernetPermissionlessValidatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkDelegatorCurrentPriority,
			expected: false,
		},
		{
			priority: PrimaryNetworkValidatorCurrentPriority,
			expected: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%d", test.priority), func(t *testing.T) {
			require.Equal(t, test.expected, test.priority.IsPendingDelegator())
		})
	}
}
