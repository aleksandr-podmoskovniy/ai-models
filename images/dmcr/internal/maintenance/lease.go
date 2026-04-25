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

package maintenance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultGateLeaseName      = "dmcr-gc-maintenance"
	GateSequenceAnnotationKey = "ai.deckhouse.io/dmcr-maintenance-sequence"
)

type LeaseGate struct {
	client    kubernetes.Interface
	namespace string
	name      string
	identity  string
	duration  time.Duration
	now       func() time.Time
}

func NewLeaseGate(client kubernetes.Interface, namespace, name, identity string, duration time.Duration) (*LeaseGate, error) {
	switch {
	case client == nil:
		return nil, fmt.Errorf("maintenance gate kubernetes client must not be nil")
	case strings.TrimSpace(namespace) == "":
		return nil, fmt.Errorf("maintenance gate namespace must not be empty")
	case strings.TrimSpace(name) == "":
		return nil, fmt.Errorf("maintenance gate lease name must not be empty")
	case strings.TrimSpace(identity) == "":
		return nil, fmt.Errorf("maintenance gate identity must not be empty")
	case duration <= 0:
		return nil, fmt.Errorf("maintenance gate duration must be greater than zero")
	}
	return &LeaseGate{
		client:    client,
		namespace: strings.TrimSpace(namespace),
		name:      strings.TrimSpace(name),
		identity:  strings.TrimSpace(identity),
		duration:  duration,
		now:       time.Now,
	}, nil
}

func (g *LeaseGate) Activate(ctx context.Context) (string, func(context.Context) error, error) {
	sequence, err := g.upsert(ctx, true)
	if err != nil {
		return "", nil, err
	}
	return sequence, g.Release, nil
}

func (g *LeaseGate) Release(ctx context.Context) error {
	_, err := g.upsert(ctx, false)
	return err
}

func (g *LeaseGate) upsert(ctx context.Context, active bool) (string, error) {
	now := g.now().UTC()
	leaseClient := g.client.CoordinationV1().Leases(g.namespace)
	lease, err := leaseClient.Get(ctx, g.name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		if !active {
			return "", nil
		}
		sequence := "1"
		_, err = leaseClient.Create(ctx, g.newLease(now), metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return g.upsert(ctx, active)
		}
		return sequence, err
	case err != nil:
		return "", fmt.Errorf("get dmcr maintenance gate lease: %w", err)
	}

	updated := lease.DeepCopy()
	sequence := leaseSequence(lease)
	if active {
		holder := leaseHolder(lease)
		if holder != "" && holder != g.identity && leaseActive(lease, now) {
			return "", fmt.Errorf("dmcr maintenance gate %s/%s is held by %s", g.namespace, g.name, holder)
		}
		sequence = nextLeaseSequence(lease)
		g.activateLease(updated, lease, now, sequence)
	} else {
		holder := leaseHolder(lease)
		if holder != "" && holder != g.identity && leaseActive(lease, now) {
			return sequence, nil
		}
		g.releaseLease(updated, now)
	}
	_, err = leaseClient.Update(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return "", fmt.Errorf("update dmcr maintenance gate lease conflict")
	}
	if err != nil {
		return "", fmt.Errorf("update dmcr maintenance gate lease: %w", err)
	}
	return sequence, nil
}

func (g *LeaseGate) newLease(now time.Time) *coordinationv1.Lease {
	microTime := metav1.NewMicroTime(now)
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        g.name,
			Namespace:   g.namespace,
			Annotations: map[string]string{GateSequenceAnnotationKey: "1"},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       stringPtr(g.identity),
			LeaseDurationSeconds: int32Ptr(leaseDurationSeconds(g.duration)),
			AcquireTime:          &microTime,
			RenewTime:            &microTime,
			LeaseTransitions:     int32Ptr(0),
		},
	}
}

func (g *LeaseGate) activateLease(updated, current *coordinationv1.Lease, now time.Time, sequence string) {
	microTime := metav1.NewMicroTime(now)
	holder := leaseHolder(current)
	if updated.Annotations == nil {
		updated.Annotations = make(map[string]string, 1)
	}
	updated.Annotations[GateSequenceAnnotationKey] = sequence
	updated.Spec.HolderIdentity = stringPtr(g.identity)
	updated.Spec.LeaseDurationSeconds = int32Ptr(leaseDurationSeconds(g.duration))
	updated.Spec.RenewTime = &microTime
	if holder != g.identity {
		updated.Spec.AcquireTime = &microTime
		updated.Spec.LeaseTransitions = int32Ptr(incrementLeaseTransitions(current))
	}
}

func (g *LeaseGate) releaseLease(updated *coordinationv1.Lease, now time.Time) {
	microTime := metav1.NewMicroTime(now)
	updated.Spec.HolderIdentity = nil
	updated.Spec.LeaseDurationSeconds = int32Ptr(1)
	updated.Spec.RenewTime = &microTime
}

func leaseActive(lease *coordinationv1.Lease, now time.Time) bool {
	if leaseHolder(lease) == "" {
		return false
	}
	return !leaseExpired(lease, now)
}

func leaseExpired(lease *coordinationv1.Lease, now time.Time) bool {
	referenceTime, ok := leaseReferenceTime(lease)
	if !ok {
		return true
	}
	duration := time.Second
	if lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds > 0 {
		duration = time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	}
	return !now.Before(referenceTime.Add(duration))
}

func leaseReferenceTime(lease *coordinationv1.Lease) (time.Time, bool) {
	if lease == nil {
		return time.Time{}, false
	}
	if lease.Spec.RenewTime != nil {
		return lease.Spec.RenewTime.Time, true
	}
	if lease.Spec.AcquireTime != nil {
		return lease.Spec.AcquireTime.Time, true
	}
	return time.Time{}, false
}

func leaseHolder(lease *coordinationv1.Lease) string {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return ""
	}
	return strings.TrimSpace(*lease.Spec.HolderIdentity)
}

func leaseSequence(lease *coordinationv1.Lease) string {
	if lease == nil {
		return ""
	}
	return strings.TrimSpace(lease.Annotations[GateSequenceAnnotationKey])
}

func nextLeaseSequence(lease *coordinationv1.Lease) string {
	current, err := strconv.ParseInt(leaseSequence(lease), 10, 64)
	if err != nil || current < 0 {
		current = 0
	}
	return strconv.FormatInt(current+1, 10)
}

func incrementLeaseTransitions(lease *coordinationv1.Lease) int32 {
	if lease == nil || lease.Spec.LeaseTransitions == nil {
		return 0
	}
	return *lease.Spec.LeaseTransitions + 1
}

func leaseDurationSeconds(duration time.Duration) int32 {
	seconds := int64(duration.Round(time.Second) / time.Second)
	if seconds <= 0 {
		return 1
	}
	return int32(seconds)
}

func stringPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}
