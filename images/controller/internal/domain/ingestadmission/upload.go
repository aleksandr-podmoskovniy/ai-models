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

package ingestadmission

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

const MaxUploadProbeBytes = 64 * 1024

type UploadSession struct {
	Owner               OwnerBinding
	Identity            publicationdata.Identity
	DeclaredInputFormat modelsv1alpha1.ModelInputFormat
	ExpectedSizeBytes   int64
}

type UploadProbeInput struct {
	FileName string
	Chunk    []byte
}

type UploadProbeResult struct {
	FileName            string
	ResolvedInputFormat modelsv1alpha1.ModelInputFormat
}

func ValidateUploadSession(session UploadSession) error {
	if err := ValidateOwnerBinding(session.Owner, session.Identity); err != nil {
		return err
	}
	if err := ValidateDeclaredInputFormat(session.DeclaredInputFormat); err != nil {
		return err
	}
	if session.ExpectedSizeBytes < 0 {
		return errors.New("upload expected size bytes must not be negative")
	}
	return nil
}

func ValidateUploadProbe(session UploadSession, probe UploadProbeInput) (UploadProbeResult, error) {
	if err := ValidateUploadSession(session); err != nil {
		return UploadProbeResult{}, err
	}

	fileName, err := normalizeFileName(probe.FileName)
	if err != nil {
		return UploadProbeResult{}, err
	}
	switch {
	case len(probe.Chunk) == 0:
		return UploadProbeResult{}, errors.New("upload probe chunk must not be empty")
	case len(probe.Chunk) > MaxUploadProbeBytes:
		return UploadProbeResult{}, fmt.Errorf("upload probe chunk must not exceed %d bytes", MaxUploadProbeBytes)
	}

	inputKind, err := classifyUploadedFile(fileName, probe.Chunk)
	if err != nil {
		return UploadProbeResult{}, err
	}

	result := UploadProbeResult{FileName: fileName}
	switch inputKind {
	case uploadedInputArchive:
		result.ResolvedInputFormat = session.DeclaredInputFormat
		return result, nil
	case uploadedInputGGUF:
		if session.DeclaredInputFormat == modelsv1alpha1.ModelInputFormatSafetensors {
			return UploadProbeResult{}, fmt.Errorf("upload file %q does not match declared input format %q", fileName, session.DeclaredInputFormat)
		}
		if session.DeclaredInputFormat != "" {
			result.ResolvedInputFormat = session.DeclaredInputFormat
		} else {
			result.ResolvedInputFormat = modelsv1alpha1.ModelInputFormatGGUF
		}
		return result, nil
	case uploadedInputSafetensors:
		return UploadProbeResult{}, errors.New("direct safetensors upload requires an archive bundle with config.json and weights")
	default:
		switch session.DeclaredInputFormat {
		case modelsv1alpha1.ModelInputFormatGGUF:
			return UploadProbeResult{}, fmt.Errorf("upload file %q does not match declared input format %q", fileName, session.DeclaredInputFormat)
		case modelsv1alpha1.ModelInputFormatSafetensors:
			return UploadProbeResult{}, errors.New("safetensors upload requires an archive bundle with config.json and weights")
		default:
			return UploadProbeResult{}, errors.New("upload probe could not determine a supported model input from file name and probe chunk")
		}
	}
}

type uploadedInputKind string

const (
	uploadedInputUnknown     uploadedInputKind = ""
	uploadedInputArchive     uploadedInputKind = "archive"
	uploadedInputGGUF        uploadedInputKind = "gguf"
	uploadedInputSafetensors uploadedInputKind = "safetensors"
)

func classifyUploadedFile(fileName string, chunk []byte) (uploadedInputKind, error) {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".zip"):
		if !looksLikeZIP(chunk) {
			return uploadedInputUnknown, fmt.Errorf("upload probe chunk does not match .zip file %q", fileName)
		}
		return uploadedInputArchive, nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		if !looksLikeGzip(chunk) {
			return uploadedInputUnknown, fmt.Errorf("upload probe chunk does not match gzip archive %q", fileName)
		}
		return uploadedInputArchive, nil
	case strings.HasSuffix(lower, ".tar"):
		return uploadedInputArchive, nil
	case strings.HasSuffix(lower, ".gguf"):
		if !hasGGUFMagic(chunk) {
			return uploadedInputUnknown, fmt.Errorf("upload probe chunk does not match .gguf file %q", fileName)
		}
		return uploadedInputGGUF, nil
	case strings.HasSuffix(lower, ".safetensors"):
		return uploadedInputSafetensors, nil
	case hasGGUFMagic(chunk):
		return uploadedInputGGUF, nil
	default:
		return uploadedInputUnknown, nil
	}
}

func hasGGUFMagic(chunk []byte) bool {
	return len(chunk) >= 4 && string(chunk[:4]) == "GGUF"
}

func looksLikeZIP(chunk []byte) bool {
	return len(chunk) >= 4 && (bytes.Equal(chunk[:4], []byte("PK\x03\x04")) ||
		bytes.Equal(chunk[:4], []byte("PK\x05\x06")) ||
		bytes.Equal(chunk[:4], []byte("PK\x07\x08")))
}

func looksLikeGzip(chunk []byte) bool {
	return len(chunk) >= 2 && chunk[0] == 0x1f && chunk[1] == 0x8b
}
