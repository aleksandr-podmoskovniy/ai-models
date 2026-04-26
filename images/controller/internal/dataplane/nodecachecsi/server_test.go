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

package nodecachecsi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodePublishVolumeBindMountsReadyDigest(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	target := filepath.Join(t.TempDir(), "pod", "volumes", "ai-models")
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	source := nodecache.StorePath(cacheRoot, digest)
	writeReadyArtifact(t, source, digest)

	mounter := &fakeMounter{}
	server := newTestServer(t, cacheRoot, mounter)
	if _, err := server.NodePublishVolume(context.Background(), publishRequest(target, digest)); err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}

	if got, want := mounter.boundSource, nodecache.SharedArtifactModelPath(cacheRoot, digest); got != want {
		t.Fatalf("bound source = %q, want %q", got, want)
	}
	if got, want := mounter.boundTarget, target; got != want {
		t.Fatalf("bound target = %q, want %q", got, want)
	}
	if !mounter.boundReadOnly {
		t.Fatal("expected read-only bind mount")
	}
	if _, ok, err := nodecache.ReadLastUsed(source); err != nil || !ok {
		t.Fatalf("expected last-used marker, ok=%t err=%v", ok, err)
	}
}

func TestNodePublishVolumeReturnsUnavailableUntilDigestReady(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	digest := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	server := newTestServer(t, cacheRoot, &fakeMounter{})

	_, err := server.NodePublishVolume(context.Background(), publishRequest("/var/lib/kubelet/pods/pod/volumes/kubernetes.io~csi/model", digest))
	if got, want := status.Code(err), codes.Unavailable; got != want {
		t.Fatalf("status.Code = %s, want %s; err=%v", got, want, err)
	}
}

func TestNodePublishVolumeDeniesUnauthorizedDigest(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	digest := "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	source := nodecache.StorePath(cacheRoot, digest)
	writeReadyArtifact(t, source, digest)

	server, err := NewServer(Options{
		NodeID:     "node-a",
		CacheRoot:  cacheRoot,
		Mounter:    &fakeMounter{},
		Authorizer: fakeAuthorizer{allowed: false},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	_, err = server.NodePublishVolume(context.Background(), publishRequest("/var/lib/kubelet/pods/pod/volumes/kubernetes.io~csi/model", digest))
	if got, want := status.Code(err), codes.PermissionDenied; got != want {
		t.Fatalf("status.Code = %s, want %s; err=%v", got, want, err)
	}
}

func TestNodePublishVolumeTreatsAuthorizerErrorsAsTransient(t *testing.T) {
	t.Parallel()

	server, err := NewServer(Options{
		NodeID:     "node-a",
		CacheRoot:  t.TempDir(),
		Mounter:    &fakeMounter{},
		Authorizer: fakeAuthorizer{err: os.ErrPermission},
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	digest := "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	_, err = server.NodePublishVolume(context.Background(), publishRequest("/var/lib/kubelet/pods/pod/volumes/kubernetes.io~csi/model", digest))
	if got, want := status.Code(err), codes.Unavailable; got != want {
		t.Fatalf("status.Code = %s, want %s; err=%v", got, want, err)
	}
}

func TestNodePublishVolumeIsIdempotentForMountedTarget(t *testing.T) {
	t.Parallel()

	cacheRoot := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	digest := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	source := nodecache.StorePath(cacheRoot, digest)
	writeReadyArtifact(t, source, digest)

	mounter := &fakeMounter{mounted: map[string]bool{target: true}}
	server := newTestServer(t, cacheRoot, mounter)
	if _, err := server.NodePublishVolume(context.Background(), publishRequest(target, digest)); err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}
	if mounter.boundTarget != "" {
		t.Fatalf("did not expect second bind mount, got target %q", mounter.boundTarget)
	}
}

func TestNodeUnpublishVolumeUnmountsAndRemovesTarget(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	mounter := &fakeMounter{mounted: map[string]bool{target: true}}
	server := newTestServer(t, t.TempDir(), mounter)

	if _, err := server.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{TargetPath: target}); err != nil {
		t.Fatalf("NodeUnpublishVolume() error = %v", err)
	}
	if got, want := mounter.unmountedTarget, target; got != want {
		t.Fatalf("unmounted target = %q, want %q", got, want)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected target removal, stat err=%v", err)
	}
}

func TestIdentityAndNodeInfo(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, t.TempDir(), &fakeMounter{})
	info, err := server.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo() error = %v", err)
	}
	if got, want := info.Name, nodecache.CSIDriverName; got != want {
		t.Fatalf("driver name = %q, want %q", got, want)
	}
	nodeInfo, err := server.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
	if err != nil {
		t.Fatalf("NodeGetInfo() error = %v", err)
	}
	if got, want := nodeInfo.NodeId, "node-a"; got != want {
		t.Fatalf("node ID = %q, want %q", got, want)
	}
}

func newTestServer(t *testing.T, cacheRoot string, mounter Mounter) *Server {
	t.Helper()

	server, err := NewServer(Options{
		NodeID:    "node-a",
		CacheRoot: cacheRoot,
		Mounter:   mounter,
	})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server
}

func publishRequest(target, digest string) *csi.NodePublishVolumeRequest {
	return &csi.NodePublishVolumeRequest{
		VolumeId:   "model-volume",
		TargetPath: target,
		Readonly:   true,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		},
		VolumeContext: map[string]string{
			nodecache.CSIAttributeArtifactURI:    "dmcr.example.test/models/demo@" + digest,
			nodecache.CSIAttributeArtifactDigest: digest,
		},
	}
}

func writeReadyArtifact(t *testing.T, destination, digest string) {
	t.Helper()

	modelPath := filepath.Join(destination, modelpack.MaterializedModelPathName)
	if err := os.MkdirAll(modelPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(model) error = %v", err)
	}
	body := []byte(`{"digest":"` + digest + `","modelPath":"` + modelPath + `","readyAt":"` + time.Now().UTC().Format(time.RFC3339) + `"}`)
	if err := os.WriteFile(nodecache.MarkerPath(destination), body, 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}
}

type fakeMounter struct {
	mounted         map[string]bool
	boundSource     string
	boundTarget     string
	boundReadOnly   bool
	unmountedTarget string
}

type fakeAuthorizer struct {
	allowed bool
	err     error
}

func (a fakeAuthorizer) AllowPublish(context.Context, map[string]string, string) (bool, error) {
	return a.allowed, a.err
}

func (m *fakeMounter) IsMountPoint(target string) (bool, error) {
	if m.mounted[target] {
		return true, nil
	}
	if _, err := os.Stat(target); err != nil {
		return false, err
	}
	return m.mounted[target], nil
}

func (m *fakeMounter) BindMount(source, target string, readOnly bool) error {
	if m.mounted == nil {
		m.mounted = map[string]bool{}
	}
	m.boundSource = source
	m.boundTarget = target
	m.boundReadOnly = readOnly
	m.mounted[target] = true
	return nil
}

func (m *fakeMounter) Unmount(target string) error {
	m.unmountedTarget = target
	m.mounted[target] = false
	return nil
}
