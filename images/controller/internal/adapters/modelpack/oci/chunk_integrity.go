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

package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func sha256DigestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifyChunkDigest(payload []byte, digest string) error {
	if got := sha256DigestBytes(payload); got != strings.TrimSpace(digest) {
		return fmt.Errorf("chunk digest mismatch: expected %q, got %q", strings.TrimSpace(digest), got)
	}
	return nil
}

func verifyFileDigest(path, digest string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	got := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if got != strings.TrimSpace(digest) {
		return fmt.Errorf("materialized file digest mismatch for %q: expected %q, got %q", path, strings.TrimSpace(digest), got)
	}
	return nil
}

func decodeStoredChunk(payload []byte, compression string) ([]byte, error) {
	switch strings.TrimSpace(compression) {
	case "", chunkCompressionNone:
		return payload, nil
	case chunkCompressionZstd:
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		return decoder.DecodeAll(payload, nil)
	default:
		return nil, fmt.Errorf("unsupported chunk compression %q", compression)
	}
}
