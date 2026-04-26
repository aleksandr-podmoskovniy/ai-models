//go:build linux

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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type defaultMounter struct{}

func (defaultMounter) IsMountPoint(target string) (bool, error) {
	if _, err := os.Stat(target); err != nil {
		return false, err
	}
	body, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	target = filepath.Clean(target)
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 4 && filepath.Clean(unescapeMountInfo(fields[4])) == target {
			return true, nil
		}
	}
	return false, nil
}

func (defaultMounter) BindMount(source, target string, readOnly bool) error {
	if err := unix.Mount(source, target, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mount %s to %s: %w", source, target, err)
	}
	if !readOnly {
		return nil
	}
	if err := unix.Mount(source, target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
		_ = unix.Unmount(target, 0)
		return fmt.Errorf("remount read-only %s: %w", target, err)
	}
	return nil
}

func (defaultMounter) Unmount(target string) error {
	if err := unix.Unmount(target, 0); err != nil {
		if errors.Is(err, unix.EINVAL) {
			return nil
		}
		return err
	}
	return nil
}

func unescapeMountInfo(value string) string {
	return mountInfoEscape.Replace(value)
}

var mountInfoEscape = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
