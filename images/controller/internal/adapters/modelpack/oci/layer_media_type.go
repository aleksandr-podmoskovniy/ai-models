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
	"fmt"
	"regexp"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

var modelPackLayerMediaTypeRegexp = regexp.MustCompile(`^application/vnd\.cncf\.model\.(\w+(?:\.\w+)?)\.v1\.(\w+)(?:\+?(\w+))?$`)

type parsedLayerMediaType struct {
	Base        modelpackports.LayerBase
	Format      modelpackports.LayerFormat
	Compression modelpackports.LayerCompression
}

func parseLayerMediaType(value string) (parsedLayerMediaType, error) {
	cleanValue := strings.TrimSpace(value)
	match := modelPackLayerMediaTypeRegexp.FindStringSubmatch(cleanValue)
	if len(match) != 4 {
		return parsedLayerMediaType{}, fmt.Errorf("unsupported ModelPack layer mediaType %q", value)
	}

	base, err := parseLayerBase(match[1])
	if err != nil {
		return parsedLayerMediaType{}, err
	}
	format, err := parseLayerFormat(match[2])
	if err != nil {
		return parsedLayerMediaType{}, err
	}
	compression, err := parseWireCompression(match[3])
	if err != nil {
		return parsedLayerMediaType{}, err
	}
	if format == modelpackports.LayerFormatRaw && compression != modelpackports.LayerCompressionNone {
		return parsedLayerMediaType{}, fmt.Errorf("raw ModelPack layer mediaType %q must not declare compression", value)
	}

	return parsedLayerMediaType{
		Base:        base,
		Format:      format,
		Compression: compression,
	}, nil
}

func buildLayerMediaType(
	base modelpackports.LayerBase,
	format modelpackports.LayerFormat,
	compression modelpackports.LayerCompression,
) (string, error) {
	if err := validatePublishLayerBase(base); err != nil {
		return "", err
	}
	if err := validatePublishLayerFormat(format); err != nil {
		return "", err
	}
	if err := validatePublishLayerCompression(compression); err != nil {
		return "", err
	}
	if format == modelpackports.LayerFormatRaw {
		if compression != "" && compression != modelpackports.LayerCompressionNone {
			return "", fmt.Errorf("raw ModelPack layer %q must not declare compression %q", base, compression)
		}
		return fmt.Sprintf("application/vnd.cncf.model.%s.v1.raw", base), nil
	}

	switch compression {
	case "", modelpackports.LayerCompressionNone:
		return fmt.Sprintf("application/vnd.cncf.model.%s.v1.tar", base), nil
	case modelpackports.LayerCompressionGzip, modelpackports.LayerCompressionGzipFastest:
		return fmt.Sprintf("application/vnd.cncf.model.%s.v1.tar+gzip", base), nil
	case modelpackports.LayerCompressionZstd:
		return fmt.Sprintf("application/vnd.cncf.model.%s.v1.tar+zstd", base), nil
	default:
		return "", fmt.Errorf("unsupported ModelPack layer compression %q", compression)
	}
}

func parseLayerBase(value string) (modelpackports.LayerBase, error) {
	switch strings.TrimSpace(value) {
	case string(modelpackports.LayerBaseModel):
		return modelpackports.LayerBaseModel, nil
	case string(modelpackports.LayerBaseModelConfig):
		return modelpackports.LayerBaseModelConfig, nil
	case string(modelpackports.LayerBaseDataset):
		return modelpackports.LayerBaseDataset, nil
	case string(modelpackports.LayerBaseCode):
		return modelpackports.LayerBaseCode, nil
	case string(modelpackports.LayerBaseDoc):
		return modelpackports.LayerBaseDoc, nil
	default:
		return "", fmt.Errorf("unsupported ModelPack layer base type %q", value)
	}
}

func parseLayerFormat(value string) (modelpackports.LayerFormat, error) {
	switch strings.TrimSpace(value) {
	case string(modelpackports.LayerFormatTar):
		return modelpackports.LayerFormatTar, nil
	case string(modelpackports.LayerFormatRaw):
		return modelpackports.LayerFormatRaw, nil
	default:
		return "", fmt.Errorf("unsupported ModelPack layer format %q", value)
	}
}

func parseWireCompression(value string) (modelpackports.LayerCompression, error) {
	switch strings.TrimSpace(value) {
	case "", string(modelpackports.LayerCompressionNone):
		return modelpackports.LayerCompressionNone, nil
	case string(modelpackports.LayerCompressionGzip):
		return modelpackports.LayerCompressionGzip, nil
	case string(modelpackports.LayerCompressionZstd):
		return modelpackports.LayerCompressionZstd, nil
	default:
		return "", fmt.Errorf("unsupported ModelPack layer compression %q", value)
	}
}

func validatePublishLayerBase(base modelpackports.LayerBase) error {
	_, err := parseLayerBase(string(base))
	return err
}

func validatePublishLayerFormat(format modelpackports.LayerFormat) error {
	_, err := parseLayerFormat(string(format))
	return err
}

func validatePublishLayerCompression(compression modelpackports.LayerCompression) error {
	switch compression {
	case "", modelpackports.LayerCompressionNone,
		modelpackports.LayerCompressionGzip,
		modelpackports.LayerCompressionGzipFastest,
		modelpackports.LayerCompressionZstd:
		return nil
	default:
		return fmt.Errorf("unsupported ModelPack layer compression %q", compression)
	}
}

func isModelLayerBase(base modelpackports.LayerBase) bool {
	return base == modelpackports.LayerBaseModel || base == modelpackports.LayerBaseModelConfig
}
