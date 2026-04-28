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

package storageusage

import "github.com/prometheus/client_golang/prometheus"

const metricNamespace = "d8_ai_models"

var (
	capacityKnownMetric = prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, "", "storage_backend_capacity_known"),
		"Whether ai-models has a configured artifact storage capacity limit.",
		nil,
		nil,
	)
	limitBytesMetric = prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, "", "storage_backend_limit_bytes"),
		"The configured ai-models artifact storage capacity limit in bytes.",
		nil,
		nil,
	)
	usedBytesMetric = prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, "", "storage_backend_used_bytes"),
		"The committed ai-models published artifact size tracked in the storage ledger.",
		nil,
		nil,
	)
	reservedBytesMetric = prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, "", "storage_backend_reserved_bytes"),
		"The ai-models artifact storage bytes reserved by active upload sessions.",
		nil,
		nil,
	)
	availableBytesMetric = prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, "", "storage_backend_available_bytes"),
		"The remaining ai-models artifact storage bytes available for new reservations.",
		nil,
		nil,
	)
)
