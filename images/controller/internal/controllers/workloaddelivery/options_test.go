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

package workloaddelivery

import (
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
)

func TestOptionsValidateAppliesModelDeliveryDefaults(t *testing.T) {
	t.Parallel()

	options := Options{
		Service: modeldelivery.ServiceOptions{
			RegistrySourceNamespace: "d8-ai-models",
		},
	}

	if err := options.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestNormalizeOptionsPreservesDeliveryAuthKey(t *testing.T) {
	t.Parallel()

	options := normalizeOptions(Options{
		Service: modeldelivery.ServiceOptions{
			ManagedCache: modeldelivery.ManagedCacheOptions{
				Enabled: true,
			},
			DeliveryAuthKey:         "  test-delivery-auth-key  ",
			RegistrySourceNamespace: "d8-ai-models",
		},
	})

	if got, want := options.Service.DeliveryAuthKey, "test-delivery-auth-key"; got != want {
		t.Fatalf("DeliveryAuthKey = %q, want %q", got, want)
	}
}
