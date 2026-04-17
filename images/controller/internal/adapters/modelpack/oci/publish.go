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
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const materializedLayerPath = "model"

type countWriter struct {
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

func streamModelLayer(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	modelDir string,
) (blobDescriptor, error) {
	info, err := os.Stat(modelDir)
	if err != nil {
		return blobDescriptor{}, err
	}

	uploadURL, err := initiateBlobUpload(ctx, client, reference, auth)
	if err != nil {
		return blobDescriptor{}, err
	}

	hasher := sha256.New()
	counter := &countWriter{}
	reader, writerPipe := io.Pipe()
	generationDone := make(chan error, 1)
	go func() {
		err := writeLayerArchive(io.MultiWriter(writerPipe, hasher, counter), modelDir, info)
		_ = writerPipe.CloseWithError(err)
		generationDone <- err
	}()

	uploadURL, err = uploadBlobChunk(ctx, client, uploadURL, auth, reader)
	if err != nil {
		_ = reader.Close()
		generationErr := <-generationDone
		if generationErr != nil {
			return blobDescriptor{}, errors.Join(err, generationErr)
		}
		return blobDescriptor{}, err
	}

	if err := <-generationDone; err != nil {
		return blobDescriptor{}, err
	}

	digest := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if err := finalizeBlobUpload(ctx, client, uploadURL, auth, digest); err != nil {
		return blobDescriptor{}, err
	}

	return blobDescriptor{
		Digest: digest,
		DiffID: digest,
		Size:   counter.n,
	}, nil
}

func writeLayerArchive(writer io.Writer, modelDir string, info os.FileInfo) error {
	tarWriter := tar.NewWriter(writer)

	if info.IsDir() {
		if err := writeRootDirHeader(tarWriter, info.Mode().Perm()); err != nil {
			return err
		}
		if err := filepath.WalkDir(modelDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == modelDir {
				return nil
			}
			return writeTarEntry(tarWriter, modelDir, path, entry)
		}); err != nil {
			return err
		}
	} else {
		if err := writeRootDirHeader(tarWriter, 0o755); err != nil {
			return err
		}
		if err := writeSingleFileEntry(tarWriter, modelDir); err != nil {
			return err
		}
	}

	return tarWriter.Close()
}

func writeTarEntry(writer *tar.Writer, root, path string, entry os.DirEntry) error {
	if entry.Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to publish symbolic link %q", path)
	}

	info, err := entry.Info()
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	if strings.HasPrefix(relative, "../") || relative == ".." {
		return fmt.Errorf("refusing to publish path outside model root: %q", path)
	}

	layerPath := materializedLayerPath + "/" + relative
	switch {
	case info.IsDir():
		return writer.WriteHeader(&tar.Header{
			Name:     layerPath + "/",
			Typeflag: tar.TypeDir,
			Mode:     int64(info.Mode().Perm()),
		})
	case info.Mode().IsRegular():
		return writeRegularFileEntry(writer, layerPath, path, info)
	default:
		return fmt.Errorf("refusing to publish unsupported file mode %q for %q", info.Mode().String(), path)
	}
}

func writeSingleFileEntry(writer *tar.Writer, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return writeRegularFileEntry(writer, materializedLayerPath+"/"+filepath.Base(path), path, info)
}

func writeRegularFileEntry(writer *tar.Writer, layerPath, sourcePath string, info os.FileInfo) error {
	header := &tar.Header{
		Name:     filepath.ToSlash(layerPath),
		Typeflag: tar.TypeReg,
		Mode:     int64(info.Mode().Perm()),
		Size:     info.Size(),
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}

	stream, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = io.Copy(writer, stream)
	return err
}

func buildConfigBlob(diffID string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"descriptor": map[string]any{
			"name": materializedLayerPath,
		},
		"modelfs": map[string]any{
			"type":    "layers",
			"diffIds": []string{strings.TrimSpace(diffID)},
		},
		"config": map[string]any{},
	})
}

func newBlobDescriptor(payload []byte) (blobDescriptor, error) {
	if len(payload) == 0 {
		return blobDescriptor{}, errors.New("blob payload must not be empty")
	}
	digestBytes := sha256.Sum256(payload)
	return blobDescriptor{
		Digest: "sha256:" + hex.EncodeToString(digestBytes[:]),
		Size:   int64(len(payload)),
	}, nil
}

func buildManifestBlob(configDescriptor, layerDescriptor blobDescriptor) ([]byte, error) {
	return json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     ManifestMediaType,
		"artifactType":  ModelPackArtifactType,
		"config": map[string]any{
			"mediaType": ModelPackConfigMediaType,
			"digest":    configDescriptor.Digest,
			"size":      configDescriptor.Size,
		},
		"layers": []map[string]any{
			{
				"mediaType": ModelPackWeightLayerType,
				"digest":    layerDescriptor.Digest,
				"size":      layerDescriptor.Size,
				"annotations": map[string]string{
					ModelPackFilepathAnnotation: materializedLayerPath,
				},
			},
		},
	})
}

func writeRootDirHeader(writer *tar.Writer, mode os.FileMode) error {
	return writer.WriteHeader(&tar.Header{
		Name:     materializedLayerPath + "/",
		Typeflag: tar.TypeDir,
		Mode:     int64(mode),
	})
}
