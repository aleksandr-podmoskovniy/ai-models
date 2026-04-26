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
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	target, digest, err := s.publishRequest(req)
	if err != nil {
		return nil, err
	}
	if err := s.authorizePublish(ctx, req.GetVolumeContext(), digest); err != nil {
		return nil, err
	}
	source, err := s.readySource(digest)
	if err != nil {
		return nil, err
	}

	mounted, err := s.options.Mounter.IsMountPoint(target)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "check node cache CSI target mount: %v", err)
	}
	if mounted {
		if err := s.touchUsage(source); err != nil {
			return nil, err
		}
		return &csi.NodePublishVolumeResponse{}, nil
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, status.Errorf(codes.Internal, "create node cache CSI target: %v", err)
	}
	if err := s.options.Mounter.BindMount(source, target, true); err != nil {
		return nil, status.Errorf(codes.Internal, "bind node cache artifact: %v", err)
	}
	if err := s.touchUsage(source); err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *Server) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "node cache CSI unpublish request must not be nil")
	}
	target, err := cleanAbsolutePath(req.GetTargetPath(), "target path")
	if err != nil {
		return nil, err
	}
	mounted, err := s.options.Mounter.IsMountPoint(target)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "check node cache CSI target mount: %v", err)
	}
	if mounted {
		if err := s.options.Mounter.Unmount(target); err != nil {
			return nil, status.Errorf(codes.Internal, "unmount node cache CSI target: %v", err)
		}
	}
	if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "remove node cache CSI target: %v", err)
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *Server) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}

func (s *Server) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: s.options.NodeID}, nil
}

func (s *Server) publishRequest(req *csi.NodePublishVolumeRequest) (string, string, error) {
	if req == nil {
		return "", "", status.Error(codes.InvalidArgument, "node cache CSI publish request must not be nil")
	}
	if strings.TrimSpace(req.GetVolumeId()) == "" {
		return "", "", status.Error(codes.InvalidArgument, "node cache CSI volume ID must not be empty")
	}
	if req.GetVolumeCapability() == nil || req.GetVolumeCapability().GetMount() == nil {
		return "", "", status.Error(codes.InvalidArgument, "node cache CSI supports only mount volume capability")
	}
	if !req.GetReadonly() {
		return "", "", status.Error(codes.InvalidArgument, "node cache CSI volume must be published read-only")
	}
	target, err := cleanAbsolutePath(req.GetTargetPath(), "target path")
	if err != nil {
		return "", "", err
	}
	digest := artifactDigest(req.GetVolumeContext())
	if digest == "" {
		digest = artifactDigest(req.GetPublishContext())
	}
	if err := validateDigest(digest); err != nil {
		return "", "", err
	}
	return target, digest, nil
}

func (s *Server) readySource(digest string) (string, error) {
	source := nodecache.StorePath(s.options.CacheRoot, digest)
	marker, err := nodecache.ReadMarker(source)
	if err != nil {
		return "", status.Errorf(codes.Internal, "read node cache marker for %s: %v", digest, err)
	}
	if marker == nil {
		return "", status.Errorf(codes.Unavailable, "node cache artifact %s is not ready", digest)
	}
	if marker.Digest != "" && marker.Digest != digest {
		return "", status.Errorf(codes.Internal, "node cache marker digest %q does not match requested %q", marker.Digest, digest)
	}
	if _, err := os.Stat(nodecache.SharedArtifactModelPath(s.options.CacheRoot, digest)); err != nil {
		if os.IsNotExist(err) {
			return "", status.Errorf(codes.Unavailable, "node cache artifact %s model path is not ready", digest)
		}
		return "", status.Errorf(codes.Internal, "stat node cache artifact %s model path: %v", digest, err)
	}
	return source, nil
}

func (s *Server) touchUsage(source string) error {
	if err := nodecache.TouchUsage(source, time.Time{}); err != nil {
		return status.Errorf(codes.Internal, "touch node cache usage: %v", err)
	}
	return nil
}

func (s *Server) authorizePublish(ctx context.Context, attributes map[string]string, digest string) error {
	if s.options.Authorizer == nil {
		return nil
	}
	allowed, err := s.options.Authorizer.AllowPublish(ctx, attributes, digest)
	if err != nil {
		return status.Errorf(codes.Unavailable, "authorize node cache CSI publish: %v", err)
	}
	if !allowed {
		return status.Error(codes.PermissionDenied, "node cache CSI publish is allowed only for managed SharedDirect pods")
	}
	return nil
}

func cleanAbsolutePath(value, field string) (string, error) {
	value = filepath.Clean(strings.TrimSpace(value))
	if value == "" || value == "." || !filepath.IsAbs(value) {
		return "", status.Errorf(codes.InvalidArgument, "node cache CSI %s must be absolute", field)
	}
	return value, nil
}

func artifactDigest(attributes map[string]string) string {
	digest := strings.TrimSpace(attributes[nodecache.CSIAttributeArtifactDigest])
	if digest != "" {
		return digest
	}
	return nodecache.DigestFromArtifactURI(attributes[nodecache.CSIAttributeArtifactURI])
}

func validateDigest(digest string) error {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return status.Error(codes.InvalidArgument, "node cache CSI artifact digest must not be empty")
	}
	if strings.ContainsAny(digest, `/\`) || digest == "." || digest == ".." {
		return status.Errorf(codes.InvalidArgument, "node cache CSI artifact digest %q is not safe", digest)
	}
	before, after, ok := strings.Cut(digest, ":")
	if !ok || before == "" || after == "" {
		return status.Errorf(codes.InvalidArgument, "node cache CSI artifact digest %q must include algorithm and encoded digest", digest)
	}
	for _, char := range after {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return status.Errorf(codes.InvalidArgument, "node cache CSI artifact digest %q is not hex encoded", digest)
		}
	}
	return nil
}
