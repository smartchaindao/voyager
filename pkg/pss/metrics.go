// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pss

import (
	"github.com/prometheus/client_golang/prometheus"
	m "github.com/yanhuangpai/voyager/pkg/metrics"
)

type metrics struct {
	TotalMessagesSentCounter prometheus.Counter
}

func newMetrics() metrics {
	subsystem := "pss"

	return metrics{
		TotalMessagesSentCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "total_message_sent",
			Help:      "Total messages sent.",
		}),
	}
}

func (s *pss) Metrics() []prometheus.Collector {
	return m.PrometheusCollectorsFromFields(s.metrics)
}
