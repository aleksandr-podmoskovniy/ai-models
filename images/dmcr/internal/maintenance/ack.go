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
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	AckLabelKey     = "ai.deckhouse.io/dmcr-maintenance-ack"
	AckGateLabelKey = "ai.deckhouse.io/dmcr-maintenance-gate"
)

var RuntimeAckComponents = []string{"dmcr", "direct-upload"}

type AckMirror struct {
	client     kubernetes.Interface
	namespace  string
	gateName   string
	identity   string
	gatePath   string
	components []string
	ttl        time.Duration
	now        func() time.Time
}

func NewAckMirror(
	client kubernetes.Interface,
	namespace string,
	gateName string,
	identity string,
	gatePath string,
	components []string,
	ttl time.Duration,
) (*AckMirror, error) {
	switch {
	case client == nil:
		return nil, fmt.Errorf("maintenance ack mirror kubernetes client must not be nil")
	case strings.TrimSpace(namespace) == "":
		return nil, fmt.Errorf("maintenance ack mirror namespace must not be empty")
	case strings.TrimSpace(gateName) == "":
		return nil, fmt.Errorf("maintenance ack mirror gate name must not be empty")
	case strings.TrimSpace(identity) == "":
		return nil, fmt.Errorf("maintenance ack mirror identity must not be empty")
	case strings.TrimSpace(gatePath) == "":
		return nil, fmt.Errorf("maintenance ack mirror gate file path must not be empty")
	case len(components) == 0:
		return nil, fmt.Errorf("maintenance ack mirror components must not be empty")
	}
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	return &AckMirror{
		client:     client,
		namespace:  strings.TrimSpace(namespace),
		gateName:   strings.TrimSpace(gateName),
		identity:   strings.TrimSpace(identity),
		gatePath:   strings.TrimSpace(gatePath),
		components: cleanComponents(components),
		ttl:        ttl,
		now:        time.Now,
	}, nil
}

func (m *AckMirror) Sync(ctx context.Context) error {
	now := m.now().UTC()
	state, active, err := readGateFile(m.gatePath, now)
	if err != nil {
		_ = m.release(ctx)
		return err
	}
	sequence := strings.TrimSpace(state.Sequence)
	if !active || sequence == "" {
		return m.release(ctx)
	}
	if ok, err := m.localAcksReady(sequence, state.ExpiresAt.UTC(), now); err != nil || !ok {
		if err != nil {
			return err
		}
		return m.release(ctx)
	}
	return m.upsert(ctx, sequence, now)
}

func (m *AckMirror) localAcksReady(sequence string, expiresAt time.Time, now time.Time) (bool, error) {
	for _, component := range m.components {
		state, err := readAckFile(ackFilePath(m.gatePath, component))
		if err != nil {
			return false, nil
		}
		if strings.TrimSpace(state.Component) != component {
			return false, nil
		}
		if strings.TrimSpace(state.Sequence) != sequence {
			return false, nil
		}
		if state.ExpiresAt.IsZero() || !now.Before(state.ExpiresAt.UTC()) {
			return false, nil
		}
		if !state.ExpiresAt.UTC().Equal(expiresAt) {
			return false, nil
		}
	}
	return true, nil
}

func (m *AckMirror) upsert(ctx context.Context, sequence string, now time.Time) error {
	leaseClient := m.client.CoordinationV1().Leases(m.namespace)
	name := m.leaseName()
	lease, err := leaseClient.Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		_, err = leaseClient.Create(ctx, m.newLease(sequence, now), metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	case err != nil:
		return fmt.Errorf("get dmcr maintenance ack lease: %w", err)
	}

	updated := lease.DeepCopy()
	renewTime := metav1.NewMicroTime(now)
	updated.Labels = ackLeaseLabels(m.gateName)
	if updated.Annotations == nil {
		updated.Annotations = make(map[string]string, 1)
	}
	updated.Annotations[GateSequenceAnnotationKey] = sequence
	updated.Spec.HolderIdentity = stringPtr(m.identity)
	updated.Spec.LeaseDurationSeconds = int32Ptr(leaseDurationSeconds(m.ttl))
	updated.Spec.RenewTime = &renewTime
	if updated.Spec.AcquireTime == nil {
		updated.Spec.AcquireTime = &renewTime
	}
	_, err = leaseClient.Update(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("update dmcr maintenance ack lease: %w", err)
	}
	return nil
}

func (m *AckMirror) release(ctx context.Context) error {
	leaseClient := m.client.CoordinationV1().Leases(m.namespace)
	lease, err := leaseClient.Get(ctx, m.leaseName(), metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("get dmcr maintenance ack lease: %w", err)
	}
	updated := lease.DeepCopy()
	now := metav1.NewMicroTime(m.now().UTC())
	updated.Spec.HolderIdentity = nil
	updated.Spec.LeaseDurationSeconds = int32Ptr(1)
	updated.Spec.RenewTime = &now
	_, err = leaseClient.Update(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (m *AckMirror) newLease(sequence string, now time.Time) *coordinationv1.Lease {
	microTime := metav1.NewMicroTime(now)
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:        m.leaseName(),
			Namespace:   m.namespace,
			Labels:      ackLeaseLabels(m.gateName),
			Annotations: map[string]string{GateSequenceAnnotationKey: sequence},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       stringPtr(m.identity),
			LeaseDurationSeconds: int32Ptr(leaseDurationSeconds(m.ttl)),
			AcquireTime:          &microTime,
			RenewTime:            &microTime,
			LeaseTransitions:     int32Ptr(0),
		},
	}
}

func (m *AckMirror) leaseName() string {
	return strings.TrimSuffix(m.gateName+"-ack-"+strings.Trim(strings.ToLower(m.identity), "."), "-")
}

func AckQuorumReady(ctx context.Context, client kubernetes.Interface, namespace, gateName, sequence string, quorum int, now time.Time) (int, error) {
	if quorum <= 0 {
		return 0, nil
	}
	selector := fmt.Sprintf("%s=true,%s=%s", AckLabelKey, AckGateLabelKey, gateName)
	leases, err := client.CoordinationV1().Leases(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return 0, fmt.Errorf("list dmcr maintenance ack leases: %w", err)
	}
	count := 0
	for i := range leases.Items {
		lease := &leases.Items[i]
		if strings.TrimSpace(lease.Annotations[GateSequenceAnnotationKey]) != strings.TrimSpace(sequence) {
			continue
		}
		if leaseActive(lease, now.UTC()) {
			count++
		}
	}
	return count, nil
}

func ackLeaseLabels(gateName string) map[string]string {
	return map[string]string{
		AckLabelKey:     "true",
		AckGateLabelKey: strings.TrimSpace(gateName),
	}
}

func cleanComponents(components []string) []string {
	cleaned := make([]string, 0, len(components))
	for _, component := range components {
		component = strings.TrimSpace(component)
		if component != "" {
			cleaned = append(cleaned, component)
		}
	}
	return cleaned
}
