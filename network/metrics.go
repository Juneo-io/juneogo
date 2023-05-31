// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/peer"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
)

type metrics struct {
	numTracked                        prometheus.Gauge
	numPeers                          prometheus.Gauge
	numSupernetPeers                  *prometheus.GaugeVec
	timeSinceLastMsgSent              prometheus.Gauge
	timeSinceLastMsgReceived          prometheus.Gauge
	sendQueuePortionFull              prometheus.Gauge
	sendFailRate                      prometheus.Gauge
	connected                         prometheus.Counter
	disconnected                      prometheus.Counter
	acceptFailed                      prometheus.Counter
	inboundConnRateLimited            prometheus.Counter
	inboundConnAllowed                prometheus.Counter
	numUselessPeerListBytes           prometheus.Counter
	nodeUptimeWeightedAverage         prometheus.Gauge
	nodeUptimeRewardingStake          prometheus.Gauge
	nodeSupernetUptimeWeightedAverage *prometheus.GaugeVec
	nodeSupernetUptimeRewardingStake  *prometheus.GaugeVec
	peerConnectedLifetimeAverage      prometheus.Gauge

	lock                       sync.RWMutex
	peerConnectedStartTimes    map[ids.NodeID]float64
	peerConnectedStartTimesSum float64
}

func newMetrics(namespace string, registerer prometheus.Registerer, initialSupernetIDs set.Set[ids.ID]) (*metrics, error) {
	m := &metrics{
		numPeers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "peers",
			Help:      "Number of network peers",
		}),
		numTracked: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "tracked",
			Help:      "Number of currently tracked IPs attempting to be connected to",
		}),
		numSupernetPeers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peers_supernet",
				Help:      "Number of peers that are validating a particular supernet",
			},
			[]string{"supernetID"},
		),
		timeSinceLastMsgReceived: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "time_since_last_msg_received",
			Help:      "Time (in ns) since the last msg was received",
		}),
		timeSinceLastMsgSent: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "time_since_last_msg_sent",
			Help:      "Time (in ns) since the last msg was sent",
		}),
		sendQueuePortionFull: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "send_queue_portion_full",
			Help:      "Percentage of use in Send Queue",
		}),
		sendFailRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "send_fail_rate",
			Help:      "Portion of messages that recently failed to be sent over the network",
		}),
		connected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "times_connected",
			Help:      "Times this node successfully completed a handshake with a peer",
		}),
		disconnected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "times_disconnected",
			Help:      "Times this node disconnected from a peer it had completed a handshake with",
		}),
		acceptFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "accept_failed",
			Help:      "Times this node's listener failed to accept an inbound connection",
		}),
		inboundConnAllowed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "inbound_conn_throttler_allowed",
			Help:      "Times this node allowed (attempted to upgrade) an inbound connection",
		}),
		numUselessPeerListBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "num_useless_peerlist_bytes",
			Help:      "Amount of useless bytes (i.e. information about nodes we already knew/don't want to connect to) received in PeerList messages",
		}),
		inboundConnRateLimited: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "inbound_conn_throttler_rate_limited",
			Help:      "Times this node rejected an inbound connection due to rate-limiting",
		}),
		nodeUptimeWeightedAverage: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "node_uptime_weighted_average",
			Help:      "This node's uptime average weighted by observing peer stakes",
		}),
		nodeUptimeRewardingStake: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "node_uptime_rewarding_stake",
			Help:      "The percentage of total stake which thinks this node is eligible for rewards",
		}),
		nodeSupernetUptimeWeightedAverage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "node_supernet_uptime_weighted_average",
				Help:      "This node's supernet uptime averages weighted by observing supernet peer stakes",
			},
			[]string{"supernetID"},
		),
		nodeSupernetUptimeRewardingStake: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "node_supernet_uptime_rewarding_stake",
				Help:      "The percentage of supernet's total stake which thinks this node is eligible for supernet's rewards",
			},
			[]string{"supernetID"},
		),
		peerConnectedLifetimeAverage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peer_connected_duration_average",
				Help:      "The average duration of all peer connections in nanoseconds",
			},
		),
		peerConnectedStartTimes: make(map[ids.NodeID]float64),
	}

	errs := wrappers.Errs{}
	errs.Add(
		registerer.Register(m.numTracked),
		registerer.Register(m.numPeers),
		registerer.Register(m.numSupernetPeers),
		registerer.Register(m.timeSinceLastMsgReceived),
		registerer.Register(m.timeSinceLastMsgSent),
		registerer.Register(m.sendQueuePortionFull),
		registerer.Register(m.sendFailRate),
		registerer.Register(m.connected),
		registerer.Register(m.disconnected),
		registerer.Register(m.acceptFailed),
		registerer.Register(m.inboundConnAllowed),
		registerer.Register(m.numUselessPeerListBytes),
		registerer.Register(m.inboundConnRateLimited),
		registerer.Register(m.nodeUptimeWeightedAverage),
		registerer.Register(m.nodeUptimeRewardingStake),
		registerer.Register(m.nodeSupernetUptimeWeightedAverage),
		registerer.Register(m.nodeSupernetUptimeRewardingStake),
		registerer.Register(m.peerConnectedLifetimeAverage),
	)

	// init supernet tracker metrics with tracked supernets
	for supernetID := range initialSupernetIDs {
		// no need to track primary network ID
		if supernetID == constants.PrimaryNetworkID {
			continue
		}
		// initialize to 0
		supernetIDStr := supernetID.String()
		m.numSupernetPeers.WithLabelValues(supernetIDStr).Set(0)
		m.nodeSupernetUptimeWeightedAverage.WithLabelValues(supernetIDStr).Set(0)
		m.nodeSupernetUptimeRewardingStake.WithLabelValues(supernetIDStr).Set(0)
	}

	return m, errs.Err
}

func (m *metrics) markConnected(peer peer.Peer) {
	m.numPeers.Inc()
	m.connected.Inc()

	trackedSupernets := peer.TrackedSupernets()
	for supernetID := range trackedSupernets {
		m.numSupernetPeers.WithLabelValues(supernetID.String()).Inc()
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	now := float64(time.Now().UnixNano())
	m.peerConnectedStartTimes[peer.ID()] = now
	m.peerConnectedStartTimesSum += now
}

func (m *metrics) markDisconnected(peer peer.Peer) {
	m.numPeers.Dec()
	m.disconnected.Inc()

	trackedSupernets := peer.TrackedSupernets()
	for supernetID := range trackedSupernets {
		m.numSupernetPeers.WithLabelValues(supernetID.String()).Dec()
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	peerID := peer.ID()
	start := m.peerConnectedStartTimes[peerID]
	m.peerConnectedStartTimesSum -= start

	delete(m.peerConnectedStartTimes, peerID)
}

func (m *metrics) updatePeerConnectionLifetimeMetrics() {
	m.lock.RLock()
	defer m.lock.RUnlock()

	avg := float64(0)
	if n := len(m.peerConnectedStartTimes); n > 0 {
		avgStartTime := m.peerConnectedStartTimesSum / float64(n)
		avg = float64(time.Now().UnixNano()) - avgStartTime
	}

	m.peerConnectedLifetimeAverage.Set(avg)
}
