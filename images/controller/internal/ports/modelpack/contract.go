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

package modelpack

import (
	"context"
	"io"
	"path/filepath"
	"strings"
)

const MaterializedModelPathName = "model"

type RegistryAuth struct {
	Username string
	Password string
	CAFile   string
	Insecure bool
}

type LayerBase string

const (
	LayerBaseModel       LayerBase = "weight"
	LayerBaseModelConfig LayerBase = "weight.config"
	LayerBaseDataset     LayerBase = "dataset"
	LayerBaseCode        LayerBase = "code"
	LayerBaseDoc         LayerBase = "doc"
)

type LayerFormat string

const (
	LayerFormatTar LayerFormat = "tar"
	LayerFormatRaw LayerFormat = "raw"
)

type LayerCompression string

const (
	LayerCompressionNone        LayerCompression = "none"
	LayerCompressionGzip        LayerCompression = "gzip"
	LayerCompressionGzipFastest LayerCompression = "gzip-fastest"
	LayerCompressionZstd        LayerCompression = "zstd"
)

type PublishLayer struct {
	SourcePath   string
	TargetPath   string
	Base         LayerBase
	Format       LayerFormat
	Compression  LayerCompression
	Archive      *PublishArchiveSource
	ObjectSource *PublishObjectSource
}

type PublishArchiveSource struct {
	StripPathPrefix string
	SelectedFiles   []string
	Reader          PublishObjectReader
	SizeBytes       int64
}

type OpenReadResult struct {
	Body      io.ReadCloser
	SizeBytes int64
	ETag      string
}

type PublishObjectReader interface {
	OpenRead(ctx context.Context, sourcePath string) (OpenReadResult, error)
}

type PublishObjectRangeReader interface {
	OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (OpenReadResult, error)
}

type PublishObjectFile struct {
	SourcePath string
	TargetPath string
	SizeBytes  int64
	ETag       string
}

type PublishObjectSource struct {
	Reader PublishObjectReader
	Files  []PublishObjectFile
}

type DirectUploadStatePhase string

const (
	DirectUploadStatePhaseIdle      DirectUploadStatePhase = "Idle"
	DirectUploadStatePhaseRunning   DirectUploadStatePhase = "Running"
	DirectUploadStatePhaseCompleted DirectUploadStatePhase = "Completed"
	DirectUploadStatePhaseFailed    DirectUploadStatePhase = "Failed"
)

type DirectUploadStateStage string

const (
	DirectUploadStateStageIdle      DirectUploadStateStage = "Idle"
	DirectUploadStateStageStarting  DirectUploadStateStage = "Starting"
	DirectUploadStateStageUploading DirectUploadStateStage = "Uploading"
	DirectUploadStateStageResumed   DirectUploadStateStage = "Resumed"
	DirectUploadStateStageSealing   DirectUploadStateStage = "Sealing"
	DirectUploadStateStageCommitted DirectUploadStateStage = "Committed"
)

type DirectUploadLayerDescriptor struct {
	Key         string           `json:"key"`
	Digest      string           `json:"digest"`
	DiffID      string           `json:"diffID"`
	SizeBytes   int64            `json:"sizeBytes"`
	MediaType   string           `json:"mediaType"`
	TargetPath  string           `json:"targetPath"`
	Base        LayerBase        `json:"base"`
	Format      LayerFormat      `json:"format"`
	Compression LayerCompression `json:"compression"`
}

type DirectUploadCurrentLayer struct {
	Key               string `json:"key"`
	SessionToken      string `json:"sessionToken"`
	PartSizeBytes     int64  `json:"partSizeBytes"`
	TotalSizeBytes    int64  `json:"totalSizeBytes"`
	UploadedSizeBytes int64  `json:"uploadedSizeBytes"`
	DigestState       []byte `json:"digestState,omitempty"`
}

type DirectUploadState struct {
	PlannedLayerCount int                           `json:"plannedLayerCount,omitempty"`
	PlannedSizeBytes  int64                         `json:"plannedSizeBytes,omitempty"`
	Phase             DirectUploadStatePhase        `json:"phase"`
	Stage             DirectUploadStateStage        `json:"stage,omitempty"`
	CompletedLayers   []DirectUploadLayerDescriptor `json:"completedLayers,omitempty"`
	CurrentLayer      *DirectUploadCurrentLayer     `json:"currentLayer,omitempty"`
	FailureMessage    string                        `json:"failureMessage,omitempty"`
}

type DirectUploadStateStore interface {
	Load(ctx context.Context) (DirectUploadState, bool, error)
	Save(ctx context.Context, state DirectUploadState) error
}

type PublishInput struct {
	ModelDir             string
	Layers               []PublishLayer
	ArtifactURI          string
	DirectUploadEndpoint string
	DirectUploadCAFile   string
	DirectUploadInsecure bool
	DirectUploadState    DirectUploadStateStore
}

type PublishResult struct {
	Reference string
	Digest    string
	MediaType string
	SizeBytes int64
}

type MaterializeInput struct {
	ArtifactURI    string
	ArtifactDigest string
	DestinationDir string
	ArtifactFamily string
}

type MaterializeResult struct {
	ModelPath  string
	Digest     string
	MediaType  string
	MarkerPath string
}

type Publisher interface {
	Publish(ctx context.Context, input PublishInput, auth RegistryAuth) (PublishResult, error)
}

type Remover interface {
	Remove(ctx context.Context, reference string, auth RegistryAuth) error
}

type Materializer interface {
	Materialize(ctx context.Context, input MaterializeInput, auth RegistryAuth) (MaterializeResult, error)
}

func MaterializedModelPath(destinationDir string) string {
	destinationDir = filepath.Clean(strings.TrimSpace(destinationDir))
	if destinationDir == "" || destinationDir == "." {
		return MaterializedModelPathName
	}

	return filepath.Join(destinationDir, MaterializedModelPathName)
}
