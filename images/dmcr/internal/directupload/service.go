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
	"errors"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
)

const DefaultBlobPartSizeBytes int64 = 8 << 20
const DefaultSessionTTL = 24 * time.Hour

type Service struct {
	backend            Backend
	authUsername       string
	authPassword       string
	tokenSecret        []byte
	rootDirectory      string
	partSizeBytes      int64
	sessionTTL         time.Duration
	verificationPolicy VerificationPolicy
	maintenanceChecker maintenance.Checker
	now                func() time.Time
}

func NewService(backend Backend, authUsername, authPassword, tokenSecret, rootDirectory string, partSizeBytes int64, sessionTTL time.Duration) (*Service, error) {
	switch {
	case backend == nil:
		return nil, errors.New("direct upload backend must not be nil")
	case strings.TrimSpace(authUsername) == "":
		return nil, errors.New("direct upload auth username must not be empty")
	case strings.TrimSpace(authPassword) == "":
		return nil, errors.New("direct upload auth password must not be empty")
	case strings.TrimSpace(tokenSecret) == "":
		return nil, errors.New("direct upload token secret must not be empty")
	}
	if partSizeBytes <= 0 {
		partSizeBytes = DefaultBlobPartSizeBytes
	}
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}
	return &Service{
		backend:            backend,
		authUsername:       strings.TrimSpace(authUsername),
		authPassword:       strings.TrimSpace(authPassword),
		tokenSecret:        []byte(strings.TrimSpace(tokenSecret)),
		rootDirectory:      strings.TrimSpace(rootDirectory),
		partSizeBytes:      partSizeBytes,
		sessionTTL:         sessionTTL,
		verificationPolicy: DefaultVerificationPolicy,
		now:                time.Now,
	}, nil
}

func (s *Service) SetVerificationPolicy(policy VerificationPolicy) error {
	normalized, err := ParseVerificationPolicy(string(policy))
	if err != nil {
		return err
	}
	s.verificationPolicy = normalized
	return nil
}

func (s *Service) SetMaintenanceChecker(checker maintenance.Checker) {
	s.maintenanceChecker = checker
}
