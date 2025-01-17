// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debugapi exposes the debug API used to
// control and analyze low-level and runtime
// features and functionalities of Voyager.
package debugapi

import (
	"crypto/ecdsa"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/yanhuangpai/voyager/pkg/accounting"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/p2p"
	"github.com/yanhuangpai/voyager/pkg/pingpong"
	"github.com/yanhuangpai/voyager/pkg/settlement"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap"
	"github.com/yanhuangpai/voyager/pkg/settlement/swap/chequebook"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/tags"
	"github.com/yanhuangpai/voyager/pkg/topology"
	"github.com/yanhuangpai/voyager/pkg/tracing"
)

// Service implements http.Handler interface to be used in HTTP server.
type Service struct {
	overlay            infinity.Address
	publicKey          ecdsa.PublicKey
	pssPublicKey       ecdsa.PublicKey
	ethereumAddress    common.Address
	p2p                p2p.DebugService
	pingpong           pingpong.Interface
	topologyDriver     topology.Driver
	storer             storage.Storer
	logger             logging.Logger
	tracer             *tracing.Tracer
	tags               *tags.Tags
	accounting         accounting.Interface
	settlement         settlement.Interface
	chequebookEnabled  bool
	chequebook         chequebook.Service
	swap               swap.ApiInterface
	corsAllowedOrigins []string
	metricsRegistry    *prometheus.Registry
	// handler is changed in the Configure method
	handler   http.Handler
	handlerMu sync.RWMutex
}

// New creates a new Debug API Service with only basic routers enabled in order
// to expose /addresses, /health endpoints, Go metrics and pprof. It is useful to expose
// these endpoints before all dependencies are configured and injected to have
// access to basic debugging tools and /health endpoint.
func New(overlay infinity.Address, publicKey, pssPublicKey ecdsa.PublicKey, ethereumAddress common.Address, logger logging.Logger, tracer *tracing.Tracer, corsAllowedOrigins []string) *Service {
	s := new(Service)
	s.overlay = overlay
	s.publicKey = publicKey
	s.pssPublicKey = pssPublicKey
	s.ethereumAddress = ethereumAddress
	s.logger = logger
	s.tracer = tracer
	s.corsAllowedOrigins = corsAllowedOrigins
	s.metricsRegistry = newMetricsRegistry()

	s.setRouter(s.newBasicRouter())

	return s
}

// Configure injects required dependencies and configuration parameters and
// constructs HTTP routes that depend on them. It is intended and safe to call
// this method only once.
func (s *Service) Configure(p2p p2p.DebugService, pingpong pingpong.Interface, topologyDriver topology.Driver, storer storage.Storer, tags *tags.Tags, accounting accounting.Interface, settlement settlement.Interface, chequebookEnabled bool, swap swap.ApiInterface, chequebook chequebook.Service) {
	s.p2p = p2p
	s.pingpong = pingpong
	s.topologyDriver = topologyDriver
	s.storer = storer
	s.tags = tags
	s.accounting = accounting
	s.settlement = settlement
	s.chequebookEnabled = chequebookEnabled
	s.chequebook = chequebook
	s.swap = swap

	s.setRouter(s.newRouter())
}

// ServeHTTP implements http.Handler interface.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// protect handler as it is changed by the Configure method
	s.handlerMu.RLock()
	h := s.handler
	s.handlerMu.RUnlock()

	h.ServeHTTP(w, r)
}
