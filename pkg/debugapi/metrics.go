// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi

import (
	"github.com/ethsana/sana"
	"github.com/ethsana/sana/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
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
			Help:      "Sana information.",
			ConstLabels: prometheus.Labels{
				"version": sana.Version,
			},
		}),
	)

	return r
}

func (s *Service) MustRegisterMetrics(cs ...prometheus.Collector) {
	s.metricsRegistry.MustRegister(cs...)
}
