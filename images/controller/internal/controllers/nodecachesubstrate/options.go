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

package nodecachesubstrate

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Options struct {
	Enabled                bool
	MaxSize                string
	StorageClassName       string
	VolumeGroupSetName     string
	VolumeGroupNameOnNode  string
	ThinPoolName           string
	NodeSelectorLabels     map[string]string
	BlockDeviceMatchLabels map[string]string
}

func (o Options) Validate() error {
	if !o.Enabled {
		return nil
	}
	switch {
	case strings.TrimSpace(o.StorageClassName) == "":
		return errors.New("node cache substrate storage class name must not be empty")
	case strings.TrimSpace(o.VolumeGroupSetName) == "":
		return errors.New("node cache substrate volume group set name must not be empty")
	case strings.TrimSpace(o.VolumeGroupNameOnNode) == "":
		return errors.New("node cache substrate VG name on node must not be empty")
	case strings.TrimSpace(o.ThinPoolName) == "":
		return errors.New("node cache substrate thin pool name must not be empty")
	case len(o.NodeSelectorLabels) == 0:
		return errors.New("node cache substrate node selector must not be empty")
	case len(o.BlockDeviceMatchLabels) == 0:
		return errors.New("node cache substrate block device selector must not be empty")
	}
	if _, err := resource.ParseQuantity(strings.TrimSpace(o.MaxSize)); err != nil {
		return errors.New("node cache substrate max size must be a valid quantity")
	}
	return nil
}
