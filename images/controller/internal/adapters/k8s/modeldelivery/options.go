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

package modeldelivery

import (
	"errors"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultInitContainerName = "ai-models-materializer"
	DefaultCacheMountPath    = "/data/modelcache"
	DefaultCurrentPathName   = "current"

	ResolvedDigestAnnotation = "ai.deckhouse.io/resolved-digest"

	LogFormatEnv     = "LOG_FORMAT"
	LogLevelEnv      = "LOG_LEVEL"
	defaultLogFormat = "json"
	defaultLogLevel  = "info"
)

type Options struct {
	RuntimeImage    string
	ImagePullPolicy corev1.PullPolicy
	LogFormat       string
	LogLevel        string
	OCIInsecure     bool

	InitContainerName string
	CacheMountPath    string
}

func NormalizeOptions(options Options) Options {
	if options.ImagePullPolicy == "" {
		options.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if strings.TrimSpace(options.LogFormat) == "" {
		options.LogFormat = defaultLogFormat
	}
	if strings.TrimSpace(options.LogLevel) == "" {
		options.LogLevel = defaultLogLevel
	}
	if strings.TrimSpace(options.InitContainerName) == "" {
		options.InitContainerName = DefaultInitContainerName
	}
	if strings.TrimSpace(options.CacheMountPath) == "" {
		options.CacheMountPath = DefaultCacheMountPath
	}
	return options
}

func ValidateOptions(options Options) error {
	switch {
	case strings.TrimSpace(options.RuntimeImage) == "":
		return errors.New("runtime delivery image must not be empty")
	case strings.TrimSpace(options.InitContainerName) == "":
		return errors.New("runtime delivery init container name must not be empty")
	case strings.TrimSpace(options.CacheMountPath) == "":
		return errors.New("runtime delivery cache mount path must not be empty")
	case !strings.HasPrefix(strings.TrimSpace(options.CacheMountPath), "/"):
		return errors.New("runtime delivery cache mount path must be absolute")
	}
	return nil
}

func CurrentModelPath(options Options) string {
	return filepath.Join(strings.TrimSpace(options.CacheMountPath), DefaultCurrentPathName)
}
