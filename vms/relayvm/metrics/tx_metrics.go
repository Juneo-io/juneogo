// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

var _ txs.Visitor = (*txMetrics)(nil)

type txMetrics struct {
	numAddDelegatorTxs,
	numAddSupernetValidatorTxs,
	numAddValidatorTxs,
	numAdvanceTimeTxs,
	numCreateChainTxs,
	numCreateSupernetTxs,
	numExportTxs,
	numImportTxs,
	numRewardValidatorTxs,
	numRemoveSupernetValidatorTxs,
	numTransformSupernetTxs,
	numAddPermissionlessValidatorTxs,
	numAddPermissionlessDelegatorTxs prometheus.Counter
}

func newTxMetrics(
	namespace string,
	registerer prometheus.Registerer,
) (*txMetrics, error) {
	errs := wrappers.Errs{}
	m := &txMetrics{
		numAddDelegatorTxs:               newTxMetric(namespace, "add_delegator", registerer, &errs),
		numAddSupernetValidatorTxs:       newTxMetric(namespace, "add_supernet_validator", registerer, &errs),
		numAddValidatorTxs:               newTxMetric(namespace, "add_validator", registerer, &errs),
		numAdvanceTimeTxs:                newTxMetric(namespace, "advance_time", registerer, &errs),
		numCreateChainTxs:                newTxMetric(namespace, "create_chain", registerer, &errs),
		numCreateSupernetTxs:             newTxMetric(namespace, "create_supernet", registerer, &errs),
		numExportTxs:                     newTxMetric(namespace, "export", registerer, &errs),
		numImportTxs:                     newTxMetric(namespace, "import", registerer, &errs),
		numRewardValidatorTxs:            newTxMetric(namespace, "reward_validator", registerer, &errs),
		numRemoveSupernetValidatorTxs:    newTxMetric(namespace, "remove_supernet_validator", registerer, &errs),
		numTransformSupernetTxs:          newTxMetric(namespace, "transform_supernet", registerer, &errs),
		numAddPermissionlessValidatorTxs: newTxMetric(namespace, "add_permissionless_validator", registerer, &errs),
		numAddPermissionlessDelegatorTxs: newTxMetric(namespace, "add_permissionless_delegator", registerer, &errs),
	}
	return m, errs.Err
}

func newTxMetric(
	namespace string,
	txName string,
	registerer prometheus.Registerer,
	errs *wrappers.Errs,
) prometheus.Counter {
	txMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      fmt.Sprintf("%s_txs_accepted", txName),
		Help:      fmt.Sprintf("Number of %s transactions accepted", txName),
	})
	errs.Add(registerer.Register(txMetric))
	return txMetric
}

func (m *txMetrics) AddValidatorTx(*txs.AddValidatorTx) error {
	m.numAddValidatorTxs.Inc()
	return nil
}

func (m *txMetrics) AddSupernetValidatorTx(*txs.AddSupernetValidatorTx) error {
	m.numAddSupernetValidatorTxs.Inc()
	return nil
}

func (m *txMetrics) AddDelegatorTx(*txs.AddDelegatorTx) error {
	m.numAddDelegatorTxs.Inc()
	return nil
}

func (m *txMetrics) CreateChainTx(*txs.CreateChainTx) error {
	m.numCreateChainTxs.Inc()
	return nil
}

func (m *txMetrics) CreateSupernetTx(*txs.CreateSupernetTx) error {
	m.numCreateSupernetTxs.Inc()
	return nil
}

func (m *txMetrics) ImportTx(*txs.ImportTx) error {
	m.numImportTxs.Inc()
	return nil
}

func (m *txMetrics) ExportTx(*txs.ExportTx) error {
	m.numExportTxs.Inc()
	return nil
}

func (m *txMetrics) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	m.numAdvanceTimeTxs.Inc()
	return nil
}

func (m *txMetrics) RewardValidatorTx(*txs.RewardValidatorTx) error {
	m.numRewardValidatorTxs.Inc()
	return nil
}

func (m *txMetrics) RemoveSupernetValidatorTx(*txs.RemoveSupernetValidatorTx) error {
	m.numRemoveSupernetValidatorTxs.Inc()
	return nil
}

func (m *txMetrics) TransformSupernetTx(*txs.TransformSupernetTx) error {
	m.numTransformSupernetTxs.Inc()
	return nil
}

func (m *txMetrics) AddPermissionlessValidatorTx(*txs.AddPermissionlessValidatorTx) error {
	m.numAddPermissionlessValidatorTxs.Inc()
	return nil
}

func (m *txMetrics) AddPermissionlessDelegatorTx(*txs.AddPermissionlessDelegatorTx) error {
	m.numAddPermissionlessDelegatorTxs.Inc()
	return nil
}
