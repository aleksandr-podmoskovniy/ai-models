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
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type directUploadMultipartTarget struct {
	ObjectKey string
	UploadID  string
}

type MultipartUploadInventoryEntry struct {
	Prefix      string
	ObjectKey   string
	UploadID    string
	PartCount   int
	InitiatedAt time.Time
}

type directUploadMultipartInventory struct {
	StoredUploadCount int
	StoredPartCount   int
	StaleUploads      []MultipartUploadInventoryEntry
}

func discoverDirectUploadMultipartInventory(
	ctx context.Context,
	store prefixStore,
	rootDirectory string,
	now time.Time,
	policy cleanupPolicy,
) (directUploadMultipartInventory, error) {
	policy = normalizeCleanupPolicy(policy)

	var (
		storedPartCount int
		staleUploads    []MultipartUploadInventoryEntry
		cutoff          = now.UTC().Add(-policy.directUploadStaleAge)
	)
	uploads := make([]multipartUploadInfo, 0, 16)
	if err := store.ForEachMultipartUpload(ctx, directUploadInventoryBasePrefix(rootDirectory), func(upload multipartUploadInfo) {
		uploads = append(uploads, upload)
	}); err != nil {
		return directUploadMultipartInventory{}, fmt.Errorf("discover direct-upload multipart uploads: %w", err)
	}

	sort.Slice(uploads, func(i, j int) bool {
		if uploads[i].Key == uploads[j].Key {
			return uploads[i].UploadID < uploads[j].UploadID
		}
		return uploads[i].Key < uploads[j].Key
	})

	for _, upload := range uploads {
		if upload.InitiatedAt.IsZero() {
			return directUploadMultipartInventory{}, fmt.Errorf("multipart upload %s (%s) is missing initiated timestamp", strings.Trim(strings.TrimSpace(upload.Key), "/"), strings.TrimSpace(upload.UploadID))
		}

		partCount, err := store.CountMultipartUploadParts(ctx, upload.Key, upload.UploadID)
		if err != nil {
			if errors.Is(err, errMultipartUploadGone) {
				continue
			}
			return directUploadMultipartInventory{}, formatMultipartUploadTargetError(upload.Key, upload.UploadID, err)
		}
		storedPartCount += partCount

		target := normalizeDirectUploadMultipartTarget(directUploadMultipartTarget{
			ObjectKey: upload.Key,
			UploadID:  upload.UploadID,
		})
		if !policy.allowImmediateDirectUploadCleanup {
			if _, targeted := policy.targetDirectUploadMultipartUploads[target]; !targeted && upload.InitiatedAt.After(cutoff) {
				continue
			}
		}

		prefix, ok := inferDirectUploadPrefix(rootDirectory, upload.Key)
		if !ok {
			return directUploadMultipartInventory{}, fmt.Errorf("multipart upload %s (%s) does not point to a valid direct-upload object prefix", strings.Trim(strings.TrimSpace(upload.Key), "/"), strings.TrimSpace(upload.UploadID))
		}
		staleUploads = append(staleUploads, MultipartUploadInventoryEntry{
			Prefix:      prefix,
			ObjectKey:   strings.Trim(strings.TrimSpace(upload.Key), "/"),
			UploadID:    strings.TrimSpace(upload.UploadID),
			PartCount:   partCount,
			InitiatedAt: upload.InitiatedAt.UTC(),
		})
	}

	return directUploadMultipartInventory{
		StoredUploadCount: len(uploads),
		StoredPartCount:   storedPartCount,
		StaleUploads:      staleUploads,
	}, nil
}

func normalizeDirectUploadMultipartTarget(target directUploadMultipartTarget) directUploadMultipartTarget {
	return directUploadMultipartTarget{
		ObjectKey: strings.Trim(strings.TrimSpace(target.ObjectKey), "/"),
		UploadID:  strings.TrimSpace(target.UploadID),
	}
}
