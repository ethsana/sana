// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debugapi exposes the debug API used to
// control and analyze low-level and runtime
// features and functionalities of Bee.
package debugapi

import (
	"crypto/ecdsa"
	"net/http"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethsana/sana/pkg/accounting"
	"github.com/ethsana/sana/pkg/logging"
	"github.com/ethsana/sana/pkg/mine"
	"github.com/ethsana/sana/pkg/p2p"
	"github.com/ethsana/sana/pkg/pingpong"
	"github.com/ethsana/sana/pkg/postage"
	"github.com/ethsana/sana/pkg/postage/postagecontract"
	"github.com/ethsana/sana/pkg/settlement"
	"github.com/ethsana/sana/pkg/settlement/swap"
	"github.com/ethsana/sana/pkg/settlement/swap/chequebook"
	"github.com/ethsana/sana/pkg/storage"
	"github.com/ethsana/sana/pkg/swarm"
	"github.com/ethsana/sana/pkg/tags"
	"github.com/ethsana/sana/pkg/topology"
	"github.com/ethsana/sana/pkg/topology/lightnode"
	"github.com/ethsana/sana/pkg/tracing"
	"github.com/ethsana/sana/pkg/transaction"
	"github.com/prometheus/client_golang/prometheus"
)

// Service implements http.Handler interface to be used in HTTP server.
type Service struct {
	overlay            *swarm.Address
	publicKey          ecdsa.PublicKey
	pssPublicKey       ecdsa.PublicKey
	ethereumAddress    common.Address
	p2p                p2p.DebugService
	pingpong           pingpong.Interface
	topologyDriver     topology.Driver
	storer             storage.Storer
	tracer             *tracing.Tracer
	tags               *tags.Tags
	accounting         accounting.Interface
	pseudosettle       settlement.Interface
	chequebookEnabled  bool
	chequebook         chequebook.Service
	swap               swap.Interface
	batchStore         postage.Storer
	transaction        transaction.Service
	post               postage.Service
	postageContract    postagecontract.Interface
	logger             logging.Logger
	corsAllowedOrigins []string
	metricsRegistry    *prometheus.Registry
	lightNodes         *lightnode.Container
	minerEnabled       bool
	mine               mine.Service
	// handler is changed in the Configure method
	handler   http.Handler
	handlerMu sync.RWMutex
}

// New creates a new Debug API Service with only basic routers enabled in order
// to expose /addresses, /health endpoints, Go metrics and pprof. It is useful to expose
// these endpoints before all dependencies are configured and injected to have
// access to basic debugging tools and /health endpoint.
func New(publicKey, pssPublicKey ecdsa.PublicKey, ethereumAddress common.Address, logger logging.Logger, tracer *tracing.Tracer, corsAllowedOrigins []string, transaction transaction.Service) *Service {
	s := new(Service)
	s.publicKey = publicKey
	s.pssPublicKey = pssPublicKey
	s.ethereumAddress = ethereumAddress
	s.logger = logger
	s.tracer = tracer
	s.corsAllowedOrigins = corsAllowedOrigins
	s.metricsRegistry = newMetricsRegistry()
	s.transaction = transaction

	s.setRouter(s.newBasicRouter())

	return s
}

// Configure injects required dependencies and configuration parameters and
// constructs HTTP routes that depend on them. It is intended and safe to call
// this method only once.
func (s *Service) Configure(overlay swarm.Address, p2p p2p.DebugService, pingpong pingpong.Interface, topologyDriver topology.Driver, lightNodes *lightnode.Container, storer storage.Storer, tags *tags.Tags, accounting accounting.Interface, pseudosettle settlement.Interface, chequebookEnabled bool, swap swap.Interface, chequebook chequebook.Service, batchStore postage.Storer, post postage.Service, postageContract postagecontract.Interface, minerEnabled bool, miner mine.Service) {
	s.p2p = p2p
	s.pingpong = pingpong
	s.topologyDriver = topologyDriver
	s.storer = storer
	s.tags = tags
	s.accounting = accounting
	s.chequebookEnabled = chequebookEnabled
	s.chequebook = chequebook
	s.swap = swap
	s.lightNodes = lightNodes
	s.batchStore = batchStore
	s.pseudosettle = pseudosettle
	s.overlay = &overlay
	s.post = post
	s.postageContract = postageContract
	s.minerEnabled = minerEnabled
	s.mine = miner

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
