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
	"time"
)

const (
	RequestLabelKey              = "ai.deckhouse.io/dmcr-gc-request"
	RequestLabelValue            = "true"
	RequestQueuedAtAnnotationKey = "ai.deckhouse.io/dmcr-gc-requested-at"
	switchAnnotationKey          = "ai.deckhouse.io/dmcr-gc-switch"
	doneAnnotationKey            = "ai.deckhouse.io/dmcr-gc-done"
	DefaultRegistryBinary        = "/usr/bin/dmcr"
	DefaultConfigPath            = "/etc/docker/registry/config.yml"
	DefaultRescanInterval        = 5 * time.Second
	DefaultActivationDelay       = 10 * time.Minute
	DefaultExecutorLeaseName     = "dmcr-gc-executor"
	DefaultExecutorLeaseDuration = 45 * time.Second
	DefaultExecutorRenewInterval = 15 * time.Second
)

type Options struct {
	RequestNamespace           string
	RequestLabelSelector       string
	RegistryBinary             string
	ConfigPath                 string
	GCTimeout                  time.Duration
	RescanInterval             time.Duration
	ActivationDelay            time.Duration
	Schedule                   string
	ExecutorLeaseName          string
	ExecutorIdentity           string
	ExecutorLeaseDuration      time.Duration
	ExecutorLeaseRenewInterval time.Duration
}

func DefaultRequestLabelSelector() string {
	return RequestLabelKey + "=" + RequestLabelValue
}

func applyDefaultOptions(options Options) Options {
	if strings.TrimSpace(options.RequestLabelSelector) == "" {
		options.RequestLabelSelector = DefaultRequestLabelSelector()
	}
	if strings.TrimSpace(options.RegistryBinary) == "" {
		options.RegistryBinary = DefaultRegistryBinary
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		options.ConfigPath = DefaultConfigPath
	}
	if options.GCTimeout <= 0 {
		options.GCTimeout = 10 * time.Minute
	}
	if options.RescanInterval <= 0 {
		options.RescanInterval = DefaultRescanInterval
	}
	if options.ActivationDelay <= 0 {
		options.ActivationDelay = DefaultActivationDelay
	}
	if strings.TrimSpace(options.ExecutorLeaseName) == "" {
		options.ExecutorLeaseName = DefaultExecutorLeaseName
	}
	if strings.TrimSpace(options.ExecutorIdentity) == "" {
		options.ExecutorIdentity = defaultExecutorIdentity()
	}
	if options.ExecutorLeaseDuration <= 0 {
		options.ExecutorLeaseDuration = DefaultExecutorLeaseDuration
	}
	if options.ExecutorLeaseRenewInterval <= 0 {
		options.ExecutorLeaseRenewInterval = DefaultExecutorRenewInterval
	}
	return options
}
