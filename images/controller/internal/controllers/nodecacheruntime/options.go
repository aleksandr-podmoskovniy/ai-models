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

package nodecacheruntime

import (
	"errors"
	"strings"
	"time"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Options struct {
	Enabled             bool
	Namespace           string
	RuntimeImage        string
	ImagePullSecretName string
	ServiceAccountName  string
	StorageClassName    string
	SharedVolumeSize    string
	MaxTotalSize        string
	MaxUnusedAge        string
	ScanInterval        string
	OCIInsecure         bool
	OCIAuthSecretName   string
	OCIRegistryCASecret string
	NodeSelectorLabels  map[string]string
}

func (o Options) Validate() error {
	if !o.Enabled {
		return nil
	}
	if err := o.validateRequiredFields(); err != nil {
		return err
	}
	return o.validateParsedValues()
}

func (o Options) MatchesNode(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	for key, value := range o.NodeSelectorLabels {
		actual, found := node.Labels[key]
		if !found || strings.TrimSpace(actual) != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func (o Options) runtimeSpec(nodeName string) k8sadapters.RuntimeSpec {
	return k8sadapters.RuntimeSpec{
		Namespace:           o.Namespace,
		NodeName:            nodeName,
		RuntimeImage:        o.RuntimeImage,
		ImagePullSecretName: o.ImagePullSecretName,
		ServiceAccountName:  o.ServiceAccountName,
		StorageClassName:    o.StorageClassName,
		SharedVolumeSize:    o.SharedVolumeSize,
		MaxTotalSize:        o.MaxTotalSize,
		MaxUnusedAge:        o.MaxUnusedAge,
		ScanInterval:        o.ScanInterval,
		OCIInsecure:         o.OCIInsecure,
		OCIAuthSecretName:   o.OCIAuthSecretName,
		OCIRegistryCASecret: o.OCIRegistryCASecret,
	}
}

func timeParseDuration(value string) (time.Duration, error) {
	if value == "" {
		return nodecache.DefaultMaintenancePeriod, nil
	}
	return time.ParseDuration(value)
}

func (o Options) validateRequiredFields() error {
	switch {
	case strings.TrimSpace(o.Namespace) == "":
		return errors.New("node cache runtime namespace must not be empty")
	case strings.TrimSpace(o.RuntimeImage) == "":
		return errors.New("node cache runtime image must not be empty")
	case strings.TrimSpace(o.ServiceAccountName) == "":
		return errors.New("node cache runtime service account must not be empty")
	case strings.TrimSpace(o.StorageClassName) == "":
		return errors.New("node cache runtime storage class name must not be empty")
	case strings.TrimSpace(o.SharedVolumeSize) == "":
		return errors.New("node cache runtime shared volume size must not be empty")
	case strings.TrimSpace(o.MaxTotalSize) == "":
		return errors.New("node cache runtime max total size must not be empty")
	case strings.TrimSpace(o.MaxUnusedAge) == "":
		return errors.New("node cache runtime max unused age must not be empty")
	case strings.TrimSpace(o.ScanInterval) == "":
		return errors.New("node cache runtime scan interval must not be empty")
	case strings.TrimSpace(o.OCIAuthSecretName) == "":
		return errors.New("node cache runtime OCI auth secret must not be empty")
	case len(o.NodeSelectorLabels) == 0:
		return errors.New("node cache runtime node selector must not be empty")
	default:
		return nil
	}
}

func (o Options) validateParsedValues() error {
	if _, err := resource.ParseQuantity(strings.TrimSpace(o.SharedVolumeSize)); err != nil {
		return errors.New("node cache runtime shared volume size must be a valid quantity")
	}
	if _, err := resource.ParseQuantity(strings.TrimSpace(o.MaxTotalSize)); err != nil {
		return errors.New("node cache runtime max total size must be a valid quantity")
	}
	if _, err := timeParseDuration(strings.TrimSpace(o.MaxUnusedAge)); err != nil {
		return errors.New("node cache runtime max unused age must be a valid duration")
	}
	if _, err := timeParseDuration(strings.TrimSpace(o.ScanInterval)); err != nil {
		return errors.New("node cache runtime scan interval must be a valid duration")
	}
	return nil
}
