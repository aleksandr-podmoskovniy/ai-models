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

package sourcefetch

import (
	"errors"
	"fmt"
	"math"
)

const (
	tarBlockSize         int64 = 512
	tarEndOfArchiveBytes int64 = 1024
	// Keep the admission estimate fail-safe: final ModelPack adds OCI config,
	// manifests and may bundle small companion files into tar layers.
	huggingFaceOCIReservationOverheadBytes int64 = 64 << 20
)

func huggingFaceStorageReservationBytes(files []RemoteObjectFile, mirror bool) (int64, error) {
	sourceBytes, err := strictRemoteObjectFilesSize(files)
	if err != nil {
		return 0, err
	}
	canonicalBytes, err := estimateHuggingFaceCanonicalArtifactBytes(files)
	if err != nil {
		return 0, err
	}
	if !mirror {
		return canonicalBytes, nil
	}
	return addStorageBytes(sourceBytes, canonicalBytes)
}

func estimateHuggingFaceCanonicalArtifactBytes(files []RemoteObjectFile) (int64, error) {
	sizeBytes := huggingFaceOCIReservationOverheadBytes
	var tarOverhead int64 = tarEndOfArchiveBytes
	for _, file := range files {
		if file.SizeBytes <= 0 {
			return 0, fmt.Errorf("huggingface file %q size is unknown", file.TargetPath)
		}
		var err error
		sizeBytes, err = addStorageBytes(sizeBytes, file.SizeBytes)
		if err != nil {
			return 0, err
		}
		tarOverhead, err = addStorageBytes(tarOverhead, tarEntryOverheadBytes(file.SizeBytes))
		if err != nil {
			return 0, err
		}
	}
	return addStorageBytes(sizeBytes, tarOverhead)
}

func strictRemoteObjectFilesSize(files []RemoteObjectFile) (int64, error) {
	if len(files) == 0 {
		return 0, errors.New("huggingface storage planning produced no files")
	}
	var total int64
	for _, file := range files {
		if file.SizeBytes <= 0 {
			return 0, fmt.Errorf("huggingface file %q size is unknown", file.TargetPath)
		}
		next, err := addStorageBytes(total, file.SizeBytes)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}

func tarEntryOverheadBytes(sizeBytes int64) int64 {
	padding := sizeBytes % tarBlockSize
	if padding > 0 {
		padding = tarBlockSize - padding
	}
	return tarBlockSize + padding
}

func addStorageBytes(left, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, errors.New("storage byte estimate must not be negative")
	}
	if left > math.MaxInt64-right {
		return 0, errors.New("storage byte estimate overflow")
	}
	return left + right, nil
}
