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

package cmdsupport

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

func ParseOptionalPositiveBytesQuantity(value, field string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid quantity", field)
	}
	bytes := quantity.Value()
	if bytes <= 0 {
		return 0, fmt.Errorf("%s must be positive", field)
	}
	return bytes, nil
}
