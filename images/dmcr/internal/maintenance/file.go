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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/leaseutil"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	GateFileEnv     = "DMCR_MAINTENANCE_GATE_FILE"
	DefaultGateFile = "/run/dmcr-maintenance/gate.json"
)

type Checker interface {
	Active(context.Context) (bool, error)
}

type fileState struct {
	ExpiresAt time.Time `json:"expiresAt"`
	Holder    string    `json:"holder,omitempty"`
	Sequence  string    `json:"sequence,omitempty"`
}

type ackState struct {
	Sequence  string    `json:"sequence"`
	Component string    `json:"component"`
	AckedAt   time.Time `json:"ackedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type FileChecker struct {
	path string
	now  func() time.Time
}

func NewFileChecker(path string) (*FileChecker, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("maintenance gate file path must not be empty")
	}
	return &FileChecker{path: strings.TrimSpace(path), now: time.Now}, nil
}

func NewFileCheckerFromEnv() (*FileChecker, error) {
	path := strings.TrimSpace(os.Getenv(GateFileEnv))
	if path == "" {
		return nil, nil
	}
	return NewFileChecker(path)
}

func (c *FileChecker) Active(context.Context) (bool, error) {
	_, active, err := readGateFile(c.path, c.now().UTC())
	return active, err
}

type FileAckObserver struct {
	gatePath  string
	ackPath   string
	component string
	interval  time.Duration
	now       func() time.Time
}

func NewFileAckObserver(gatePath, component string, interval time.Duration) (*FileAckObserver, error) {
	component = strings.TrimSpace(component)
	switch {
	case strings.TrimSpace(gatePath) == "":
		return nil, fmt.Errorf("maintenance gate file path must not be empty")
	case component == "":
		return nil, fmt.Errorf("maintenance ack component must not be empty")
	}
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	return &FileAckObserver{
		gatePath:  strings.TrimSpace(gatePath),
		ackPath:   ackFilePath(strings.TrimSpace(gatePath), component),
		component: component,
		interval:  interval,
		now:       time.Now,
	}, nil
}

func NewFileAckObserverFromEnv(component string, interval time.Duration) (*FileAckObserver, error) {
	path := strings.TrimSpace(os.Getenv(GateFileEnv))
	if path == "" {
		return nil, nil
	}
	return NewFileAckObserver(path, component, interval)
}

func (o *FileAckObserver) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()
		for {
			if err := o.Sync(); err != nil {
				slog.Default().Warn("dmcr maintenance gate ack sync failed", slog.String("component", o.component), slog.Any("error", err))
			}
			select {
			case <-ctx.Done():
				_ = removeGateFile(o.ackPath)
				return
			case <-ticker.C:
			}
		}
	}()
}

func (o *FileAckObserver) Sync() error {
	state, active, err := readGateFile(o.gatePath, o.now().UTC())
	if err != nil {
		_ = removeGateFile(o.ackPath)
		return err
	}
	if !active || strings.TrimSpace(state.Sequence) == "" {
		return removeGateFile(o.ackPath)
	}
	return writeAckFile(o.ackPath, ackState{
		Sequence:  strings.TrimSpace(state.Sequence),
		Component: o.component,
		AckedAt:   o.now().UTC(),
		ExpiresAt: state.ExpiresAt.UTC(),
	})
}

type FileMirror struct {
	client    kubernetes.Interface
	namespace string
	name      string
	path      string
	now       func() time.Time
}

func NewFileMirror(client kubernetes.Interface, namespace, name, path string) (*FileMirror, error) {
	switch {
	case client == nil:
		return nil, fmt.Errorf("maintenance gate mirror kubernetes client must not be nil")
	case strings.TrimSpace(namespace) == "":
		return nil, fmt.Errorf("maintenance gate mirror namespace must not be empty")
	case strings.TrimSpace(name) == "":
		return nil, fmt.Errorf("maintenance gate mirror lease name must not be empty")
	case strings.TrimSpace(path) == "":
		return nil, fmt.Errorf("maintenance gate mirror file path must not be empty")
	}
	return &FileMirror{
		client:    client,
		namespace: strings.TrimSpace(namespace),
		name:      strings.TrimSpace(name),
		path:      strings.TrimSpace(path),
		now:       time.Now,
	}, nil
}

func (m *FileMirror) Sync(ctx context.Context) error {
	lease, err := m.client.CoordinationV1().Leases(m.namespace).Get(ctx, m.name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return removeGateFile(m.path)
	case err != nil:
		return fmt.Errorf("get dmcr maintenance gate lease: %w", err)
	}
	if !leaseActive(lease, m.now().UTC()) {
		return removeGateFile(m.path)
	}
	expiresAt, ok := leaseExpiresAt(lease)
	if !ok {
		return fmt.Errorf("maintenance gate lease is missing expiry reference")
	}
	return writeGateFile(m.path, fileState{ExpiresAt: expiresAt, Holder: leaseutil.Holder(lease), Sequence: leaseSequence(lease)})
}

func leaseExpiresAt(lease *coordinationv1.Lease) (time.Time, bool) {
	return leaseutil.ExpiresAt(lease, time.Second, false)
}

func writeGateFile(path string, state fileState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readGateFile(path string, now time.Time) (fileState, bool, error) {
	payload, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fileState{}, false, nil
	case err != nil:
		return fileState{}, false, fmt.Errorf("read maintenance gate file: %w", err)
	}
	var state fileState
	if err := json.Unmarshal(payload, &state); err != nil {
		return fileState{}, false, fmt.Errorf("parse maintenance gate file: %w", err)
	}
	if state.ExpiresAt.IsZero() {
		return fileState{}, false, fmt.Errorf("maintenance gate file is missing expiresAt")
	}
	return state, now.UTC().Before(state.ExpiresAt.UTC()), nil
}

func writeAckFile(path string, state ackState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readAckFile(path string) (ackState, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ackState{}, err
	}
	var state ackState
	if err := json.Unmarshal(payload, &state); err != nil {
		return ackState{}, err
	}
	return state, nil
}

func ackFilePath(gatePath, component string) string {
	return filepath.Join(filepath.Dir(gatePath), "ack-"+component+".json")
}

func removeGateFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
