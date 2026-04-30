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

package logging

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

const expectedRegistryDeleteMissReason = "expected_registry_delete_miss"

// ConfigureDistributionLogFilters keeps upstream registry access logs useful
// for operators without reporting expected post-delete lookup misses as errors.
func ConfigureDistributionLogFilters() {
	logrus.AddHook(distributionResponseLogHook{})
}

type distributionResponseLogHook struct{}

func (distributionResponseLogHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel}
}

func (distributionResponseLogHook) Fire(entry *logrus.Entry) error {
	if isExpectedRegistryDeleteMiss(entry) {
		entry.Level = logrus.InfoLevel
		entry.Data["reason"] = expectedRegistryDeleteMissReason
	}
	return nil
}

func isExpectedRegistryDeleteMiss(entry *logrus.Entry) bool {
	if entry == nil || entry.Message != "response completed with error" {
		return false
	}
	if fmt.Sprint(entry.Data["http.response.status"]) != fmt.Sprint(http.StatusNotFound) {
		return false
	}
	method := strings.ToUpper(strings.TrimSpace(fmt.Sprint(entry.Data["http.request.method"])))
	if method != http.MethodDelete {
		return false
	}
	uri := strings.TrimSpace(fmt.Sprint(entry.Data["http.request.uri"]))
	if !strings.Contains(uri, "/manifests/") {
		return false
	}
	switch strings.TrimSpace(fmt.Sprint(entry.Data["err.code"])) {
	case "MANIFEST_UNKNOWN", "NAME_UNKNOWN":
		return true
	default:
		return false
	}
}
