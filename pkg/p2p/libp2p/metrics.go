// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libp2p

import (
	"github.com/prometheus/client_golang/prometheus"
	m "github.com/yanhuangpai/voyager/pkg/metrics"
)

type metrics struct {
	// all metrics fields must be exported
	// to be able to return them by Metrics()
	// using reflection
	CreatedConnectionCount  prometheus.Counter
	HandledConnectionCount  prometheus.Counter
	CreatedStreamCount      prometheus.Counter
	HandledStreamCount      prometheus.Counter
	BlocklistedPeerCount    prometheus.Counter
	BlocklistedPeerErrCount prometheus.Counter
	DisconnectCount         prometheus.Counter
	ConnectBreakerCount     prometheus.Counter
}

func newMetrics() metrics {
	subsystem := "libp2p"

	return metrics{
		CreatedConnectionCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "created_connection_count",
			Help:      "Number of initiated outgoing libp2p connections.",
		}),
		HandledConnectionCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "handled_connection_count",
			Help:      "Number of handled incoming libp2p connections.",
		}),
		CreatedStreamCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "created_stream_count",
			Help:      "Number of initiated outgoing libp2p streams.",
		}),
		HandledStreamCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "handled_stream_count",
			Help:      "Number of handled incoming libp2p streams.",
		}),
		BlocklistedPeerCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "blocklisted_peer_count",
			Help:      "Number of peers we've blocklisted.",
		}),
		BlocklistedPeerErrCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "blocklisted_peer_err_count",
			Help:      "Number of peers we've voyagern unable to blocklist.",
		}),
		DisconnectCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "disconnect_count",
			Help:      "Number of peers we've disconnected from (initiated locally).",
		}),
		ConnectBreakerCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: m.Namespace,
			Subsystem: subsystem,
			Name:      "connect_breaker_count",
			Help:      "Number of times we got a closed breaker while connecting to another peer.",
		}),
	}
}

func (s *Service) Metrics() []prometheus.Collector {
	return m.PrometheusCollectorsFromFields(s.metrics)
}
