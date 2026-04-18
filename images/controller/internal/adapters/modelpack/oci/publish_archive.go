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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func writeLayerArchive(writer io.Writer, sourcePath, targetPath string, info os.FileInfo) error {
	tarWriter := tar.NewWriter(writer)
	if info.IsDir() {
		if err := writeRootDirHeader(tarWriter, targetPath, info.Mode().Perm()); err != nil {
			return err
		}
		if err := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == sourcePath {
				return nil
			}
			return writeTarEntry(tarWriter, sourcePath, path, filepath.ToSlash(strings.TrimSpace(targetPath)), entry)
		}); err != nil {
			return err
		}
	} else {
		if err := writeSingleFileEntry(tarWriter, sourcePath, filepath.ToSlash(strings.TrimSpace(targetPath))); err != nil {
			return err
		}
	}
	return tarWriter.Close()
}

func writeTarEntry(writer *tar.Writer, root, path, targetRoot string, entry os.DirEntry) error {
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
		return fmt.Errorf("refusing to publish path outside layer root: %q", path)
	}

	layerPath := filepath.ToSlash(strings.Trim(strings.TrimSpace(targetRoot), "/"))
	if relative != "" && relative != "." {
		layerPath += "/" + relative
	}
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

func writeSingleFileEntry(writer *tar.Writer, path, targetPath string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	layerPath := filepath.ToSlash(strings.Trim(strings.TrimSpace(targetPath), "/"))
	if info.IsDir() {
		return fmt.Errorf("single file entry source %q must not be a directory", path)
	}
	return writeRegularFileEntry(writer, layerPath, path, info)
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

func writeRootDirHeader(writer *tar.Writer, rootPath string, mode os.FileMode) error {
	return writer.WriteHeader(&tar.Header{
		Name:     filepath.ToSlash(strings.Trim(strings.TrimSpace(rootPath), "/")) + "/",
		Typeflag: tar.TypeDir,
		Mode:     int64(mode),
	})
}
