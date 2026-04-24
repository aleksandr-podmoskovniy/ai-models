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

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func parsePositiveQuantity(flagName, value string) (resource.Quantity, error) {
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("%s: %w", flagName, err)
	}
	if quantity.Sign() <= 0 {
		return resource.Quantity{}, fmt.Errorf("%s must be greater than zero", flagName)
	}
	return quantity, nil
}

func buildPublicationWorkerResources(
	cpuRequest string,
	cpuLimit string,
	memoryRequest string,
	memoryLimit string,
	ephemeralRequest string,
	ephemeralLimit string,
) (corev1.ResourceRequirements, error) {
	requestCPU, err := parsePositiveQuantity("publication-worker-cpu-request", cpuRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitCPU, err := parsePositiveQuantity("publication-worker-cpu-limit", cpuLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	requestMemory, err := parsePositiveQuantity("publication-worker-memory-request", memoryRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitMemory, err := parsePositiveQuantity("publication-worker-memory-limit", memoryLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	requestEphemeral, err := parsePositiveQuantity("publication-worker-ephemeral-storage-request", ephemeralRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitEphemeral, err := parsePositiveQuantity("publication-worker-ephemeral-storage-limit", ephemeralLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              requestCPU,
			corev1.ResourceMemory:           requestMemory,
			corev1.ResourceEphemeralStorage: requestEphemeral,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              limitCPU,
			corev1.ResourceMemory:           limitMemory,
			corev1.ResourceEphemeralStorage: limitEphemeral,
		},
	}, nil
}

func parseMatchLabelsJSON(raw string) (map[string]string, error) {
	raw = normalizeMatchLabelsJSON(raw)
	labels := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &labels); err != nil {
		return nil, fmt.Errorf("parse matchLabels json: %w", err)
	}
	return labels, nil
}

func normalizeMatchLabelsJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return raw
	}
	if raw[0] == '"' && raw[len(raw)-1] == '"' {
		unquoted, err := strconv.Unquote(raw)
		if err == nil {
			return unquoted
		}
		return raw[1 : len(raw)-1]
	}
	if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1]
	}
	return raw
}
