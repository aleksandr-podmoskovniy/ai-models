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

package leaseutil

import (
	"math"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
)

func Holder(lease *coordinationv1.Lease) string {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return ""
	}
	return strings.TrimSpace(*lease.Spec.HolderIdentity)
}

func Expired(lease *coordinationv1.Lease, now time.Time, fallbackDuration time.Duration, includeCreationTimestamp bool) bool {
	referenceTime, ok := ReferenceTime(lease, includeCreationTimestamp)
	if !ok {
		return true
	}
	duration := Duration(lease, fallbackDuration)
	if duration <= 0 {
		return true
	}
	return !referenceTime.Add(duration).After(now.UTC())
}

func ExpiresAt(lease *coordinationv1.Lease, fallbackDuration time.Duration, includeCreationTimestamp bool) (time.Time, bool) {
	referenceTime, ok := ReferenceTime(lease, includeCreationTimestamp)
	if !ok {
		return time.Time{}, false
	}
	duration := Duration(lease, fallbackDuration)
	if duration <= 0 {
		return time.Time{}, false
	}
	return referenceTime.Add(duration), true
}

func ReferenceTime(lease *coordinationv1.Lease, includeCreationTimestamp bool) (time.Time, bool) {
	if lease == nil {
		return time.Time{}, false
	}
	if lease.Spec.RenewTime != nil {
		return lease.Spec.RenewTime.Time.UTC(), true
	}
	if lease.Spec.AcquireTime != nil {
		return lease.Spec.AcquireTime.Time.UTC(), true
	}
	if includeCreationTimestamp && !lease.CreationTimestamp.IsZero() {
		return lease.CreationTimestamp.Time.UTC(), true
	}
	return time.Time{}, false
}

func Duration(lease *coordinationv1.Lease, fallback time.Duration) time.Duration {
	if lease != nil && lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds > 0 {
		return time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	}
	return fallback
}

func DurationSeconds(duration, fallback time.Duration) int32 {
	if duration <= 0 {
		duration = fallback
	}
	seconds := int32(math.Ceil(duration.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func StringPtr(value string) *string {
	return &value
}

func Int32Ptr(value int32) *int32 {
	return &value
}
