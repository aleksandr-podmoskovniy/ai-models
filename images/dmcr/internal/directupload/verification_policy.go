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

package directupload

import (
	"fmt"
	"strings"
)

type VerificationPolicy string

const (
	VerificationPolicyTrustedBackendOrClientAsserted VerificationPolicy = "trusted-backend-or-client-asserted"
	VerificationPolicyTrustedBackendOrReread         VerificationPolicy = "trusted-backend-or-reread"
	DefaultVerificationPolicy                                           = VerificationPolicyTrustedBackendOrClientAsserted
)

func ParseVerificationPolicy(raw string) (VerificationPolicy, error) {
	switch strings.TrimSpace(raw) {
	case "":
		return DefaultVerificationPolicy, nil
	case string(VerificationPolicyTrustedBackendOrClientAsserted):
		return VerificationPolicyTrustedBackendOrClientAsserted, nil
	case string(VerificationPolicyTrustedBackendOrReread):
		return VerificationPolicyTrustedBackendOrReread, nil
	default:
		return "", fmt.Errorf(
			"unsupported direct upload verification policy %q, want %q or %q",
			strings.TrimSpace(raw),
			VerificationPolicyTrustedBackendOrClientAsserted,
			VerificationPolicyTrustedBackendOrReread,
		)
	}
}

func (p VerificationPolicy) requiresObjectRereadFallback() bool {
	return p == VerificationPolicyTrustedBackendOrReread
}
