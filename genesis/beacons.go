// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/sampler"
)

// getIPs returns the beacon IPs for each network
func getIPs(networkID uint32) []string {
	switch networkID {
	case constants.MainnetID:
		return nil
	case constants.TestnetID:
		return []string{
			"154.53.35.120:9651",
			"38.105.232.16:9651",
			"172.104.142.98:9651",
			"172.104.150.114:9651",
			"38.242.232.146:9651",
			"172.232.42.69:9651",
			"207.180.206.236:9651",
			"178.18.254.16:9651",
			"75.119.133.213:9651",
			"161.97.153.18:9651",
			"161.97.145.106:9651",
			"66.94.121.32:9651",
			"144.126.144.62:9651",
			"194.163.188.66:9651",
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
			"NodeID-B2GHMQ8GF6FyrvmPUX6miaGeuVLH9UwHr",
			"NodeID-3G1TKgx8yxLLoAAcgd4DUG19bYX9A6xdL",
			"NodeID-5bziGPQUGgM6c1d24yCPgtriw7dW2uQUt",
			"NodeID-P6qNB7Zk2tUirf9TvBiXxiCHxa5Hzq6sL",
			"NodeID-JUuB3sUuqwg4qE7mo8v5NzFsLFiUporq5",
			"NodeID-NSn5tiuaCSzDDe2f9WQQXpWu6S2BuK79Q",
			"NodeID-H5ckHgw7zgPkSu1y2kM5V13qd4mfRQXMB",
			"NodeID-DNQ4Az5jEGTyssfUzPUCqkVF5ka5CWkXB",
			"NodeID-AYRHrMe65S2Uqe2r17gCNcK8uYnSaXrgB",
			"NodeID-JttGf5ixpbpuT4xXB8owDBBpDgtRpV1p3",
			"NodeID-Kb3CHWpkcQtKyWSw1ZfzYrRhdaDiv7efA",
			"NodeID-A7ERghFEviaetvWsU59ktJabL6btoKFnc",
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
