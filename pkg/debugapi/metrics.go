// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/yanhuangpai/voyager"
	"github.com/yanhuangpai/voyager/pkg/metrics"
)

func newMetricsRegistry() (r *prometheus.Registry) {
	r = prometheus.NewRegistry()

	// register standard metrics
	r.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
			Namespace: metrics.Namespace,
		}),
		prometheus.NewGoCollector(),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Name:      "info",
			Help:      "Voyager information.",
			ConstLabels: prometheus.Labels{
				"version": voyager.Version,
			},
		}),
	)

	return r
}

func (s *Service) MustRegisterMetrics(cs ...prometheus.Collector) {
	s.metricsRegistry.MustRegister(cs...)
}
