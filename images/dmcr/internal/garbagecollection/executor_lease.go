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

package garbagecollection

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type executorLeaseRunner struct {
	client        kubernetes.Interface
	namespace     string
	name          string
	identity      string
	duration      time.Duration
	renewInterval time.Duration
	now           func() time.Time
}

func newExecutorLeaseRunner(
	client kubernetes.Interface,
	options Options,
	now func() time.Time,
) (*executorLeaseRunner, error) {
	if client == nil {
		return nil, fmt.Errorf("kubernetes client must not be nil")
	}

	namespace := strings.TrimSpace(options.RequestNamespace)
	if namespace == "" {
		return nil, fmt.Errorf("executor lease namespace must not be empty")
	}
	name := strings.TrimSpace(options.ExecutorLeaseName)
	if name == "" {
		return nil, fmt.Errorf("executor lease name must not be empty")
	}
	identity := strings.TrimSpace(options.ExecutorIdentity)
	if identity == "" {
		return nil, fmt.Errorf("executor lease identity must not be empty")
	}

	if now == nil {
		now = time.Now
	}

	return &executorLeaseRunner{
		client:        client,
		namespace:     namespace,
		name:          name,
		identity:      identity,
		duration:      options.ExecutorLeaseDuration,
		renewInterval: options.ExecutorLeaseRenewInterval,
		now:           now,
	}, nil
}

func defaultExecutorIdentity() string {
	hostname, err := os.Hostname()
	if err == nil && strings.TrimSpace(hostname) != "" {
		return strings.TrimSpace(hostname)
	}
	return "dmcr-gc-executor"
}

func (r *executorLeaseRunner) RunIfHolder(
	ctx context.Context,
	work func(context.Context) (bool, error),
) (bool, error) {
	if work == nil {
		return false, fmt.Errorf("executor lease work must not be nil")
	}

	acquired, err := r.acquireOrRenew(ctx)
	if err != nil {
		return false, err
	}
	if !acquired {
		return false, nil
	}

	workContext, cancelWork := context.WithCancel(ctx)
	renewErrors := make(chan error, 1)
	go func() {
		err := r.renewUntilDone(workContext)
		if err != nil {
			cancelWork()
		}
		renewErrors <- err
	}()

	handled, workErr := work(workContext)
	cancelWork()
	renewErr := <-renewErrors
	if renewErr != nil {
		return handled, renewErr
	}
	if workErr != nil {
		return handled, workErr
	}
	return handled, nil
}

func (r *executorLeaseRunner) renewUntilDone(ctx context.Context) error {
	if r.renewInterval <= 0 {
		return nil
	}

	ticker := time.NewTicker(r.renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			acquired, err := r.acquireOrRenew(ctx)
			if ctx.Err() != nil {
				return nil
			}
			if err != nil {
				return err
			}
			if !acquired {
				return fmt.Errorf("dmcr garbage-collection executor lease %s/%s is no longer held by %s", r.namespace, r.name, r.identity)
			}
		}
	}
}

func (r *executorLeaseRunner) acquireOrRenew(ctx context.Context) (bool, error) {
	now := r.now().UTC()
	leaseClient := r.client.CoordinationV1().Leases(r.namespace)

	lease, err := leaseClient.Get(ctx, r.name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		_, err = leaseClient.Create(ctx, r.newLease(now), metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("create dmcr garbage-collection executor lease: %w", err)
		}
		return true, nil
	case err != nil:
		return false, fmt.Errorf("get dmcr garbage-collection executor lease: %w", err)
	}

	holder := leaseHolder(lease)
	if holder != "" && holder != r.identity && !leaseExpired(lease, now, r.duration) {
		return false, nil
	}

	updated := lease.DeepCopy()
	durationSeconds := leaseDurationSeconds(r.duration)
	renewTime := metav1.NewMicroTime(now)
	updated.Spec.HolderIdentity = stringPtr(r.identity)
	updated.Spec.LeaseDurationSeconds = int32Ptr(durationSeconds)
	updated.Spec.RenewTime = &renewTime
	if holder != r.identity {
		updated.Spec.AcquireTime = &renewTime
		updated.Spec.LeaseTransitions = int32Ptr(incrementLeaseTransitions(lease))
	} else if updated.Spec.AcquireTime == nil {
		updated.Spec.AcquireTime = &renewTime
	}

	_, err = leaseClient.Update(ctx, updated, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("update dmcr garbage-collection executor lease: %w", err)
	}
	return true, nil
}

func (r *executorLeaseRunner) newLease(now time.Time) *coordinationv1.Lease {
	durationSeconds := leaseDurationSeconds(r.duration)
	microTime := metav1.NewMicroTime(now)

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.name,
			Namespace: r.namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       stringPtr(r.identity),
			LeaseDurationSeconds: int32Ptr(durationSeconds),
			AcquireTime:          &microTime,
			RenewTime:            &microTime,
			LeaseTransitions:     int32Ptr(0),
		},
	}
}

func leaseHolder(lease *coordinationv1.Lease) string {
	if lease == nil || lease.Spec.HolderIdentity == nil {
		return ""
	}
	return strings.TrimSpace(*lease.Spec.HolderIdentity)
}

func leaseExpired(lease *coordinationv1.Lease, now time.Time, fallbackDuration time.Duration) bool {
	referenceTime, ok := leaseReferenceTime(lease)
	if !ok {
		return true
	}

	duration := fallbackDuration
	if lease != nil && lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds > 0 {
		duration = time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second
	}
	if duration <= 0 {
		return true
	}
	return !referenceTime.Add(duration).After(now.UTC())
}

func leaseReferenceTime(lease *coordinationv1.Lease) (time.Time, bool) {
	if lease == nil {
		return time.Time{}, false
	}
	if lease.Spec.RenewTime != nil {
		return lease.Spec.RenewTime.Time.UTC(), true
	}
	if lease.Spec.AcquireTime != nil {
		return lease.Spec.AcquireTime.Time.UTC(), true
	}
	if !lease.CreationTimestamp.IsZero() {
		return lease.CreationTimestamp.Time.UTC(), true
	}
	return time.Time{}, false
}

func incrementLeaseTransitions(lease *coordinationv1.Lease) int32 {
	if lease == nil || lease.Spec.LeaseTransitions == nil {
		return 1
	}
	return *lease.Spec.LeaseTransitions + 1
}

func leaseDurationSeconds(duration time.Duration) int32 {
	if duration <= 0 {
		duration = DefaultExecutorLeaseDuration
	}
	seconds := int32(math.Ceil(duration.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func stringPtr(value string) *string {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}
