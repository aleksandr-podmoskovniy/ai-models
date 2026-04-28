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

package nodecache

import "fmt"

func RequiredSizeBytes(artifacts []DesiredArtifact) int64 {
	normalized, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil {
		return 0
	}
	var total int64
	for _, artifact := range normalized {
		if artifact.SizeBytes > 0 {
			total += artifact.SizeBytes
		}
	}
	return total
}

func ValidateDesiredArtifactsFit(limitBytes int64, artifacts []DesiredArtifact) error {
	if limitBytes <= 0 || len(artifacts) == 0 {
		return nil
	}
	normalized, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil {
		return err
	}
	for _, artifact := range normalized {
		if artifact.SizeBytes <= 0 {
			return fmt.Errorf("node cache capacity cannot be checked without artifact size for digest %q", artifact.Digest)
		}
	}
	required := RequiredSizeBytes(normalized)
	if required > limitBytes {
		return fmt.Errorf("node cache capacity exceeded: requested=%d limit=%d", required, limitBytes)
	}
	return nil
}
