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

package nodecache

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

type PrefetchFunc func(context.Context, DesiredArtifact, string) error

const (
	DefaultPrefetchInitialBackoff = 30 * time.Second
	DefaultPrefetchMaxBackoff     = 5 * time.Minute
)

type PrefetchRetryOptions struct {
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Now            func() time.Time
}

type PrefetchRetryState struct {
	options  PrefetchRetryOptions
	failures map[string]prefetchFailure
}

type prefetchFailure struct {
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
}

func NewPrefetchRetryState(options PrefetchRetryOptions) *PrefetchRetryState {
	options = normalizePrefetchRetryOptions(options)
	return &PrefetchRetryState{
		options:  options,
		failures: map[string]prefetchFailure{},
	}
}

func EnsureDesiredArtifacts(
	ctx context.Context,
	cacheRoot string,
	artifacts []DesiredArtifact,
	run PrefetchFunc,
) error {
	if run == nil {
		return errors.New("node cache prefetch function must not be nil")
	}
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		return errors.New("node cache prefetch cache-root must not be empty")
	}
	artifacts, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil || len(artifacts) == 0 {
		return err
	}

	snapshot, err := Scan(cacheRoot)
	if err != nil {
		return err
	}
	ready := readyDigestSet(snapshot)

	for _, artifact := range artifacts {
		if _, found := ready[artifact.Digest]; found {
			continue
		}
		destinationDir := StorePath(cacheRoot, artifact.Digest)
		slog.Default().Info(
			"node cache prefetch started",
			slog.String("artifactURI", artifact.ArtifactURI),
			slog.String("artifactDigest", artifact.Digest),
			slog.String("destinationDir", destinationDir),
		)
		if err := run(ctx, artifact, destinationDir); err != nil {
			return err
		}
	}
	return nil
}

func EnsureDesiredArtifactsWithRetry(
	ctx context.Context,
	cacheRoot string,
	artifacts []DesiredArtifact,
	run PrefetchFunc,
	state *PrefetchRetryState,
) error {
	if run == nil {
		return errors.New("node cache prefetch function must not be nil")
	}
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		return errors.New("node cache prefetch cache-root must not be empty")
	}
	artifacts, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil || len(artifacts) == 0 {
		if state != nil {
			state.prune(nil, nil)
		}
		return err
	}
	if state == nil {
		state = NewPrefetchRetryState(PrefetchRetryOptions{})
	}

	snapshot, err := Scan(cacheRoot)
	if err != nil {
		return err
	}
	ready := readyDigestSet(snapshot)
	desired := desiredDigestSet(artifacts)
	state.prune(desired, ready)

	for _, artifact := range artifacts {
		if _, found := ready[artifact.Digest]; found {
			continue
		}
		now := state.now()
		if next, blocked := state.nextAttempt(artifact.Digest, now); blocked {
			slog.Default().Info(
				"node cache prefetch delayed by retry backoff",
				slog.String("artifactURI", artifact.ArtifactURI),
				slog.String("artifactDigest", artifact.Digest),
				slog.Time("nextAttemptAt", next),
			)
			continue
		}

		destinationDir := StorePath(cacheRoot, artifact.Digest)
		slog.Default().Info(
			"node cache prefetch started",
			slog.String("artifactURI", artifact.ArtifactURI),
			slog.String("artifactDigest", artifact.Digest),
			slog.String("destinationDir", destinationDir),
		)
		if err := run(ctx, artifact, destinationDir); err != nil {
			state.recordFailure(artifact.Digest, now, err)
			slog.Default().Warn(
				"node cache prefetch failed; digest will be retried",
				slog.String("artifactURI", artifact.ArtifactURI),
				slog.String("artifactDigest", artifact.Digest),
				slog.Any("error", err),
			)
			continue
		}
		state.recordSuccess(artifact.Digest)
	}
	return nil
}

func readyDigestSet(snapshot Snapshot) map[string]struct{} {
	ready := make(map[string]struct{}, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if !entry.Ready {
			continue
		}
		ready[entry.Digest] = struct{}{}
	}
	return ready
}

func desiredDigestSet(artifacts []DesiredArtifact) map[string]struct{} {
	desired := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		desired[artifact.Digest] = struct{}{}
	}
	return desired
}

func normalizePrefetchRetryOptions(options PrefetchRetryOptions) PrefetchRetryOptions {
	if options.InitialBackoff <= 0 {
		options.InitialBackoff = DefaultPrefetchInitialBackoff
	}
	if options.MaxBackoff <= 0 {
		options.MaxBackoff = DefaultPrefetchMaxBackoff
	}
	if options.MaxBackoff < options.InitialBackoff {
		options.MaxBackoff = options.InitialBackoff
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return options
}

func (s *PrefetchRetryState) now() time.Time {
	return s.options.Now().UTC()
}

func (s *PrefetchRetryState) nextAttempt(digest string, now time.Time) (time.Time, bool) {
	if s == nil || len(s.failures) == 0 {
		return time.Time{}, false
	}
	failure, found := s.failures[strings.TrimSpace(digest)]
	if !found || failure.NextAttemptAt.IsZero() || !now.Before(failure.NextAttemptAt) {
		return time.Time{}, false
	}
	return failure.NextAttemptAt, true
}

func (s *PrefetchRetryState) recordFailure(digest string, now time.Time, err error) {
	if s == nil {
		return
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return
	}
	failure := s.failures[digest]
	failure.Attempts++
	failure.LastError = err.Error()
	failure.NextAttemptAt = now.Add(s.backoff(failure.Attempts))
	s.failures[digest] = failure
}

func (s *PrefetchRetryState) recordSuccess(digest string) {
	if s == nil {
		return
	}
	delete(s.failures, strings.TrimSpace(digest))
}

func (s *PrefetchRetryState) prune(desired, ready map[string]struct{}) {
	if s == nil || len(s.failures) == 0 {
		return
	}
	for digest := range s.failures {
		if _, found := ready[digest]; found {
			delete(s.failures, digest)
			continue
		}
		if _, found := desired[digest]; !found {
			delete(s.failures, digest)
		}
	}
}

func (s *PrefetchRetryState) backoff(attempts int) time.Duration {
	if attempts <= 1 {
		return s.options.InitialBackoff
	}
	backoff := s.options.InitialBackoff
	for i := 1; i < attempts; i++ {
		backoff *= 2
		if backoff >= s.options.MaxBackoff {
			return s.options.MaxBackoff
		}
	}
	return backoff
}
