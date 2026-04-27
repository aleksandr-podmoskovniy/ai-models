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

package garbagecollection

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type requestState uint8

const (
	requestStateIgnored requestState = iota
	requestStateQueued
	requestStateActive
	requestStateCompleted
)

func classifyRequest(secret corev1.Secret) requestState {
	if secret.Labels[RequestLabelKey] != RequestLabelValue {
		return requestStateIgnored
	}
	if strings.TrimSpace(secret.Annotations[phaseAnnotationKey]) == phaseDone {
		return requestStateCompleted
	}
	if strings.TrimSpace(secret.Annotations[switchAnnotationKey]) != "" {
		return requestStateActive
	}
	if strings.TrimSpace(secret.Annotations[RequestQueuedAtAnnotationKey]) != "" {
		return requestStateQueued
	}
	return requestStateIgnored
}

func isCompletedRequest(secret corev1.Secret) bool {
	return classifyRequest(secret) == requestStateCompleted
}

func isQueuedRequest(secret corev1.Secret) bool {
	return classifyRequest(secret) == requestStateQueued
}

func shouldRunGarbageCollection(secret corev1.Secret) bool {
	return classifyRequest(secret) == requestStateActive
}

func hasPendingRequest(secrets []corev1.Secret) bool {
	for _, secret := range secrets {
		switch classifyRequest(secret) {
		case requestStateQueued, requestStateActive:
			return true
		}
	}
	return false
}

func queuedRequestSecrets(secrets []corev1.Secret) []corev1.Secret {
	return requestSecretsByState(secrets, requestStateQueued)
}

func activeRequestSecrets(secrets []corev1.Secret) []corev1.Secret {
	return requestSecretsByState(secrets, requestStateActive)
}

func requestSecretsByState(secrets []corev1.Secret, state requestState) []corev1.Secret {
	matched := make([]corev1.Secret, 0, len(secrets))
	for _, secret := range secrets {
		if classifyRequest(secret) == state {
			matched = append(matched, secret)
		}
	}
	return matched
}

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		names = append(names, secret.Name)
	}
	return names
}
