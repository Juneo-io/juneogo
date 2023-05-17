// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/sampler"
)

// getIPs returns the beacon IPs for each network
func getIPs(networkID uint32) []string {
	switch networkID {
	case constants.MainnetID:
		return nil
	case constants.TestnetID:
		return []string{
			"65.108.7.242:9651",
			"65.108.15.231:9651",
			"38.242.232.146:9651",
			"65.20.115.188:9651",
			"172.104.142.98:9651",
		}
	default:
		return nil
	}
}

// getNodeIDs returns the beacon node IDs for each network
func getNodeIDs(networkID uint32) []string {
	switch networkID {
	case constants.MainnetID:
		return nil
	case constants.TestnetID:
		return []string{
			"NodeID-8pi4CuccGXxg7b1BiVFez78cC75zts7qy",
			"NodeID-5SngxwT4WZV5xCkzYTGqUZWq5jybJmkSN",
			"NodeID-5bziGPQUGgM6c1d24yCPgtriw7dW2uQUt",
			"NodeID-3G1TKgx8yxLLoAAcgd4DUG19bYX9A6xdL",
			"NodeID-B2GHMQ8GF6FyrvmPUX6miaGeuVLH9UwHr",
		}
	default:
		return nil
	}
}

// SampleBeacons returns the some beacons this node should connect to
func SampleBeacons(networkID uint32, count int) ([]string, []string) {
	ips := getIPs(networkID)
	ids := getNodeIDs(networkID)

	if numIPs := len(ips); numIPs < count {
		count = numIPs
	}

	sampledIPs := make([]string, 0, count)
	sampledIDs := make([]string, 0, count)

	s := sampler.NewUniform()
	_ = s.Initialize(uint64(len(ips)))
	indices, _ := s.Sample(count)
	for _, index := range indices {
		sampledIPs = append(sampledIPs, ips[int(index)])
		sampledIDs = append(sampledIDs, ids[int(index)])
	}

	return sampledIPs, sampledIDs
}
