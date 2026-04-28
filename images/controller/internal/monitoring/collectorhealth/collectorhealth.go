/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package collectorhealth

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricNamespace = "d8_ai_models"
	unknownName     = "unknown"
)

var (
	upName          = prometheus.BuildFQName(metricNamespace, "", "collector_up")
	durationName    = prometheus.BuildFQName(metricNamespace, "", "collector_scrape_duration_seconds")
	lastSuccessName = prometheus.BuildFQName(metricNamespace, "", "collector_last_success_timestamp_seconds")
)

type State struct {
	name        string
	upDesc      *prometheus.Desc
	duration    *prometheus.Desc
	lastSuccess *prometheus.Desc
	now         func() time.Time
	mu          sync.Mutex
	lastValue   float64
}

func New(name string) *State {
	name = strings.TrimSpace(name)
	if name == "" {
		name = unknownName
	}
	labels := prometheus.Labels{"collector": name}
	return &State{
		name: name,
		upDesc: prometheus.NewDesc(
			upName,
			"Whether the ai-models collector completed its last scrape without internal collection errors.",
			nil,
			labels,
		),
		duration: prometheus.NewDesc(
			durationName,
			"Duration of the last ai-models collector scrape attempt.",
			nil,
			labels,
		),
		lastSuccess: prometheus.NewDesc(
			lastSuccessName,
			"Unix timestamp of the last successful ai-models collector scrape. Zero means no successful scrape has completed yet.",
			nil,
			labels,
		),
		now: time.Now,
	}
}

func (s *State) Describe(ch chan<- *prometheus.Desc) {
	if s == nil {
		return
	}
	ch <- s.upDesc
	ch <- s.duration
	ch <- s.lastSuccess
}

func (s *State) Report(ch chan<- prometheus.Metric, startedAt time.Time, success bool) {
	if s == nil {
		return
	}

	now := s.now()
	durationSeconds := now.Sub(startedAt).Seconds()
	if durationSeconds < 0 {
		durationSeconds = 0
	}

	s.mu.Lock()
	if success {
		s.lastValue = float64(now.UnixNano()) / float64(time.Second)
	}
	lastSuccess := s.lastValue
	s.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(s.upDesc, prometheus.GaugeValue, boolFloat64(success))
	ch <- prometheus.MustNewConstMetric(s.duration, prometheus.GaugeValue, durationSeconds)
	ch <- prometheus.MustNewConstMetric(s.lastSuccess, prometheus.GaugeValue, lastSuccess)
}

func boolFloat64(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
