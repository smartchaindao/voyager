// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package accounting

import (
	"github.com/prometheus/client_golang/prometheus"
	m "github.com/yanhuangpai/voyager/pkg/metrics"
)

type metrics struct {
	// all metrics fields must be exported
	// to be able to return them by Metrics()
	// using reflection
	TotalDebitedAmount         prometheus.Counter
	TotalCreditedAmount        prometheus.Counter
	DebitEventsCount           prometheus.Counter
	CreditEventsCount          prometheus.Counter
	AccountingDisconnectsCount prometheus.Counter
	AccountingBlocksCount      prometheus.Counter
}

func newMetrics() metrics {
	subsystem := "accounting"

	return metrics{
		TotalDebitedAmount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "total_debited_amount",
			Help:      "Amount of IFI debited to peers (potential income of the node)",
		}),
		TotalCreditedAmount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "total_credited_amount",
			Help:      "Amount of IFI credited to peers (potential cost of the node)",
		}),
		DebitEventsCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "debit_events_count",
			Help:      "Number of occurrences of IFI debit events towards peers",
		}),
		CreditEventsCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "credit_events_count",
			Help:      "Number of occurrences of IFI credit events towards peers",
		}),
		AccountingDisconnectsCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "accounting_disconnects_count",
			Help:      "Number of occurrences of peers disconnected based on payment thresholds",
		}),
		AccountingBlocksCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "accounting_blocks_count",
			Help:      "Number of occurrences of temporarily skipping a peer to avoid crossing their disconnect thresholds",
		}),
	}
}

// Metrics returns the prometheus Collector for the accounting service.
func (a *Accounting) Metrics() []prometheus.Collector {
	return m.PrometheusCollectorsFromFields(a.metrics)
}
