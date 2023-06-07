// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/metric"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/platformvm/blocks"
)

var _ Metrics = (*metrics)(nil)

type Metrics interface {
	metric.APIInterceptor

	// Mark that an option vote that we initially preferred was accepted.
	MarkOptionVoteWon()
	// Mark that an option vote that we initially preferred was rejected.
	MarkOptionVoteLost()
	// Mark that the given block was accepted.
	MarkAccepted(blocks.Block) error
	// Mark that a validator set was created.
	IncValidatorSetsCreated()
	// Mark that a validator set was cached.
	IncValidatorSetsCached()
	// Mark that we spent the given time computing validator diffs.
	AddValidatorSetsDuration(time.Duration)
	// Mark that we computed a validator diff at a height with the given
	// difference from the top.
	AddValidatorSetsHeightDiff(uint64)
	// Mark that this much stake is staked on the node.
	SetLocalStake(uint64)
	// Mark that this much stake is staked in the network.
	SetTotalStake(uint64)
	// Mark when this node will unstake from the Primary Network.
	SetTimeUntilUnstake(time.Duration)
	// Mark when this node will unstake from a supernet.
	SetTimeUntilSupernetUnstake(supernetID ids.ID, timeUntilUnstake time.Duration)
	// Mark that this node is connected to this percent of a supernet's stake.
	SetSupernetPercentConnected(supernetID ids.ID, percent float64)
	// Mark that this node is connected to this percent of the Primary Network's
	// stake.
	SetPercentConnected(percent float64)
}

func New(
	namespace string,
	registerer prometheus.Registerer,
	trackedSupernets set.Set[ids.ID],
) (Metrics, error) {
	blockMetrics, err := newBlockMetrics(namespace, registerer)
	m := &metrics{
		blockMetrics: blockMetrics,

		percentConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "percent_connected",
			Help:      "Percent of connected stake",
		}),
		supernetPercentConnected: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "percent_connected_supernet",
				Help:      "Percent of connected supernet weight",
			},
			[]string{"supernetID"},
		),
		timeUntilUnstake: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "time_until_unstake",
			Help:      "Time (in ns) until this node leaves the Primary Network's validator set",
		}),
		timeUntilSupernetUnstake: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "time_until_unstake_supernet",
				Help:      "Time (in ns) until this node leaves the supernet's validator set",
			},
			[]string{"supernetID"},
		),
		localStake: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "local_staked",
			Help:      "Amount (in nAVAX) of AVAX staked on this node",
		}),
		totalStake: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "total_staked",
			Help:      "Amount (in nAVAX) of AVAX staked on the Primary Network",
		}),

		numVotesWon: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "votes_won",
			Help:      "Total number of votes this node has won",
		}),
		numVotesLost: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "votes_lost",
			Help:      "Total number of votes this node has lost",
		}),

		validatorSetsCached: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "validator_sets_cached",
			Help:      "Total number of validator sets cached",
		}),
		validatorSetsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "validator_sets_created",
			Help:      "Total number of validator sets created from applying difflayers",
		}),
		validatorSetsHeightDiff: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "validator_sets_height_diff_sum",
			Help:      "Total number of validator sets diffs applied for generating validator sets",
		}),
		validatorSetsDuration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "validator_sets_duration_sum",
			Help:      "Total amount of time generating validator sets in nanoseconds",
		}),
	}

	errs := wrappers.Errs{Err: err}
	apiRequestMetrics, err := metric.NewAPIInterceptor(namespace, registerer)
	m.APIInterceptor = apiRequestMetrics
	errs.Add(
		err,

		registerer.Register(m.percentConnected),
		registerer.Register(m.supernetPercentConnected),
		registerer.Register(m.timeUntilUnstake),
		registerer.Register(m.timeUntilSupernetUnstake),
		registerer.Register(m.localStake),
		registerer.Register(m.totalStake),

		registerer.Register(m.numVotesWon),
		registerer.Register(m.numVotesLost),

		registerer.Register(m.validatorSetsCreated),
		registerer.Register(m.validatorSetsCached),
		registerer.Register(m.validatorSetsHeightDiff),
		registerer.Register(m.validatorSetsDuration),
	)

	// init supernet tracker metrics with tracked supernets
	for supernetID := range trackedSupernets {
		// initialize to 0
		m.supernetPercentConnected.WithLabelValues(supernetID.String()).Set(0)
	}
	return m, errs.Err
}

type metrics struct {
	metric.APIInterceptor

	blockMetrics *blockMetrics

	percentConnected         prometheus.Gauge
	supernetPercentConnected *prometheus.GaugeVec
	timeUntilUnstake         prometheus.Gauge
	timeUntilSupernetUnstake *prometheus.GaugeVec
	localStake               prometheus.Gauge
	totalStake               prometheus.Gauge

	numVotesWon, numVotesLost prometheus.Counter

	validatorSetsCached     prometheus.Counter
	validatorSetsCreated    prometheus.Counter
	validatorSetsHeightDiff prometheus.Gauge
	validatorSetsDuration   prometheus.Gauge
}

func (m *metrics) MarkOptionVoteWon() {
	m.numVotesWon.Inc()
}

func (m *metrics) MarkOptionVoteLost() {
	m.numVotesLost.Inc()
}

func (m *metrics) MarkAccepted(b blocks.Block) error {
	return b.Visit(m.blockMetrics)
}

func (m *metrics) IncValidatorSetsCreated() {
	m.validatorSetsCreated.Inc()
}

func (m *metrics) IncValidatorSetsCached() {
	m.validatorSetsCached.Inc()
}

func (m *metrics) AddValidatorSetsDuration(d time.Duration) {
	m.validatorSetsDuration.Add(float64(d))
}

func (m *metrics) AddValidatorSetsHeightDiff(d uint64) {
	m.validatorSetsHeightDiff.Add(float64(d))
}

func (m *metrics) SetLocalStake(s uint64) {
	m.localStake.Set(float64(s))
}

func (m *metrics) SetTotalStake(s uint64) {
	m.totalStake.Set(float64(s))
}

func (m *metrics) SetTimeUntilUnstake(timeUntilUnstake time.Duration) {
	m.timeUntilUnstake.Set(float64(timeUntilUnstake))
}

func (m *metrics) SetTimeUntilSupernetUnstake(supernetID ids.ID, timeUntilUnstake time.Duration) {
	m.timeUntilSupernetUnstake.WithLabelValues(supernetID.String()).Set(float64(timeUntilUnstake))
}

func (m *metrics) SetSupernetPercentConnected(supernetID ids.ID, percent float64) {
	m.supernetPercentConnected.WithLabelValues(supernetID.String()).Set(percent)
}

func (m *metrics) SetPercentConnected(percent float64) {
	m.percentConnected.Set(percent)
}
