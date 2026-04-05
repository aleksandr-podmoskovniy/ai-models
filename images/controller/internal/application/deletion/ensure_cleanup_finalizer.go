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

package deletion

type EnsureCleanupFinalizerInput struct {
	HasFinalizer bool
	HandleFound  bool
	HandleErr    error
}

type EnsureCleanupFinalizerDecision struct {
	AddFinalizer    bool
	RemoveFinalizer bool
}

func EnsureCleanupFinalizer(input EnsureCleanupFinalizerInput) (EnsureCleanupFinalizerDecision, error) {
	if input.HandleErr != nil {
		return EnsureCleanupFinalizerDecision{}, input.HandleErr
	}

	switch {
	case !input.HandleFound && input.HasFinalizer:
		return EnsureCleanupFinalizerDecision{RemoveFinalizer: true}, nil
	case !input.HandleFound:
		return EnsureCleanupFinalizerDecision{}, nil
	case input.HasFinalizer:
		return EnsureCleanupFinalizerDecision{}, nil
	default:
		return EnsureCleanupFinalizerDecision{AddFinalizer: true}, nil
	}
}
