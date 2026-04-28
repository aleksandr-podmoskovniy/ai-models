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

import "testing"

func TestParseOptionalPositiveBytesQuantity(t *testing.T) {
	value, err := ParseOptionalPositiveBytesQuantity("1Gi", "limit")
	if err != nil {
		t.Fatalf("ParseOptionalPositiveBytesQuantity() error = %v", err)
	}
	if value != 1024*1024*1024 {
		t.Fatalf("value = %d, want 1Gi in bytes", value)
	}

	value, err = ParseOptionalPositiveBytesQuantity("", "limit")
	if err != nil {
		t.Fatalf("empty quantity error = %v", err)
	}
	if value != 0 {
		t.Fatalf("empty quantity value = %d, want 0", value)
	}
}
