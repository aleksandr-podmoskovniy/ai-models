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

package s3

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func validateStatInput(input uploadstagingports.StatInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validateDownloadInput(input uploadstagingports.DownloadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.DestinationPath) == "":
		return errors.New("upload staging destination path must not be empty")
	default:
		return nil
	}
}

func validateOpenReadInput(input uploadstagingports.OpenReadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validateOpenReadRangeInput(input uploadstagingports.OpenReadRangeInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case input.Offset < 0:
		return errors.New("upload staging read offset must not be negative")
	case input.Length == 0:
		return errors.New("upload staging read length must not be zero")
	case input.Length < -1:
		return errors.New("upload staging read length must be positive or -1")
	default:
		return nil
	}
}

func objectRangeHeader(offset, length int64) (string, bool) {
	if offset <= 0 && length < 0 {
		return "", false
	}
	if length < 0 {
		return "bytes=" + strconv.FormatInt(offset, 10) + "-", true
	}
	return "bytes=" + strconv.FormatInt(offset, 10) + "-" + strconv.FormatInt(offset+length-1, 10), true
}

func validateUploadInput(input uploadstagingports.UploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case input.Body == nil:
		return errors.New("upload staging body must not be nil")
	default:
		return nil
	}
}

func validateDeleteInput(input uploadstagingports.DeleteInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validateDeletePrefixInput(input uploadstagingports.DeletePrefixInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Prefix) == "":
		return errors.New("upload staging prefix must not be empty")
	default:
		return nil
	}
}

func deletePrefixErrors(errors []types.Error) error {
	if len(errors) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errors))
	for _, entry := range errors {
		key := strings.TrimSpace(aws.ToString(entry.Key))
		code := strings.TrimSpace(aws.ToString(entry.Code))
		message := strings.TrimSpace(aws.ToString(entry.Message))
		switch {
		case key != "" && code != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s (%s: %s)", key, code, message))
		case key != "" && code != "":
			messages = append(messages, fmt.Sprintf("%s (%s)", key, code))
		case key != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s (%s)", key, message))
		case key != "":
			messages = append(messages, key)
		case code != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s: %s", code, message))
		case code != "":
			messages = append(messages, code)
		case message != "":
			messages = append(messages, message)
		default:
			messages = append(messages, "unknown deleteObjects error")
		}
	}

	return fmt.Errorf("delete prefix returned object errors: %s", strings.Join(messages, ", "))
}
