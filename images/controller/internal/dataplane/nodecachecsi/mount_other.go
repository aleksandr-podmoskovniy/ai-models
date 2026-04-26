//go:build !linux

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

package nodecachecsi

import (
	"errors"
	"os"
)

type defaultMounter struct{}

func (defaultMounter) IsMountPoint(target string) (bool, error) {
	if _, err := os.Stat(target); err != nil {
		return false, err
	}
	return false, nil
}

func (defaultMounter) BindMount(string, string, bool) error {
	return errors.New("node cache CSI bind mount is supported only on linux")
}

func (defaultMounter) Unmount(string) error {
	return errors.New("node cache CSI unmount is supported only on linux")
}
