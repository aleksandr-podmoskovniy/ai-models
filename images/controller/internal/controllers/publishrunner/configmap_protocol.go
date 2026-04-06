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

package publishrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	managedLabelValue = "true"

	managedLabelKey   = "ai-models.deckhouse.io/publication-operation"
	ownerKindLabelKey = "ai-models.deckhouse.io/owner-kind"
	ownerUIDLabelKey  = "ai-models.deckhouse.io/owner-uid"

	phaseAnnotationKey   = "ai-models.deckhouse.io/publication-phase"
	messageAnnotationKey = "ai-models.deckhouse.io/publication-message"
	workerAnnotationKey  = "ai-models.deckhouse.io/publication-worker"

	requestDataKey       = "request.json"
	resultDataKey        = "result.json"
	uploadDataKey        = "upload.json"
	workerResultDataKey  = "worker-result.json"
	workerFailureDataKey = "worker-failure.txt"
)

func NewConfigMap(namespace string, request publicationports.Request) (*corev1.ConfigMap, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, errors.New("publication operation namespace must not be empty")
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}

	name, err := resourcenames.PublicationOperationConfigMapName(request.Owner.UID)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				managedLabelKey:   managedLabelValue,
				ownerKindLabelKey: request.Owner.Kind,
				ownerUIDLabelKey:  string(request.Owner.UID),
			},
			Annotations: map[string]string{
				phaseAnnotationKey: string(publicationports.PhasePending),
			},
		},
		Data: map[string]string{
			requestDataKey: string(payload),
		},
	}, nil
}

func IsManagedConfigMap(configMap *corev1.ConfigMap) bool {
	if configMap == nil {
		return false
	}
	return configMap.Labels[managedLabelKey] == managedLabelValue
}

func RequestFromConfigMap(configMap *corev1.ConfigMap) (publicationports.Request, error) {
	return decodeRequiredConfigMapJSON(configMap, requestDataKey, "request", func(request *publicationports.Request) error {
		return request.Validate()
	})
}

func ResultFromConfigMap(configMap *corev1.ConfigMap) (publicationports.Result, error) {
	return decodeRequiredConfigMapJSON(configMap, resultDataKey, "result", func(result *publicationports.Result) error {
		return result.Validate()
	})
}

func WorkerResultFromConfigMap(configMap *corev1.ConfigMap) string {
	return optionalConfigMapData(configMap, workerResultDataKey)
}

func WorkerFailureFromConfigMap(configMap *corev1.ConfigMap) string {
	return optionalConfigMapData(configMap, workerFailureDataKey)
}

func UploadStatusFromConfigMap(configMap *corev1.ConfigMap) (*modelsv1alpha1.ModelUploadStatus, error) {
	return decodeOptionalConfigMapJSON(configMap, uploadDataKey, func(status *modelsv1alpha1.ModelUploadStatus) error {
		return validateUploadStatus(*status)
	})
}

func StatusFromConfigMap(configMap *corev1.ConfigMap) publicationports.Status {
	if configMap == nil {
		return publicationports.Status{}
	}

	phase := publicationports.Phase(strings.TrimSpace(configMap.Annotations[phaseAnnotationKey]))
	if phase == "" {
		phase = publicationports.PhasePending
	}

	return publicationports.Status{
		Phase:      phase,
		Message:    strings.TrimSpace(configMap.Annotations[messageAnnotationKey]),
		WorkerName: strings.TrimSpace(configMap.Annotations[workerAnnotationKey]),
	}
}

func SetRunning(configMap *corev1.ConfigMap, workerName string) error {
	if strings.TrimSpace(workerName) == "" {
		return errors.New("publication operation worker name must not be empty")
	}
	if err := prepareConfigMapMutation(configMap); err != nil {
		return err
	}

	setPhaseAnnotations(configMap, publicationports.PhaseRunning, workerName, "")
	return nil
}

func SetUploadReady(configMap *corev1.ConfigMap, upload modelsv1alpha1.ModelUploadStatus) error {
	if err := validateUploadStatus(upload); err != nil {
		return err
	}
	if err := prepareConfigMapMutation(configMap); err != nil {
		return err
	}

	return setConfigMapJSON(configMap, uploadDataKey, upload)
}

func SetFailed(configMap *corev1.ConfigMap, message string) error {
	if err := prepareConfigMapMutation(configMap); err != nil {
		return err
	}

	setPhaseAnnotations(configMap, publicationports.PhaseFailed, "", message)
	deleteConfigMapData(configMap, uploadDataKey, resultDataKey)
	return nil
}

func SetSucceeded(configMap *corev1.ConfigMap, result publicationports.Result) error {
	if err := result.Validate(); err != nil {
		return err
	}
	if err := prepareConfigMapMutation(configMap); err != nil {
		return err
	}

	setPhaseAnnotations(configMap, publicationports.PhaseSucceeded, "", "")
	deleteConfigMapData(configMap, uploadDataKey)
	return setConfigMapJSON(configMap, resultDataKey, result)
}

func validateUploadStatus(status modelsv1alpha1.ModelUploadStatus) error {
	if strings.TrimSpace(status.Command) == "" {
		return errors.New("publication operation upload command must not be empty")
	}
	if strings.TrimSpace(status.Repository) == "" {
		return errors.New("publication operation upload repository must not be empty")
	}
	if status.ExpiresAt == nil || status.ExpiresAt.IsZero() {
		return errors.New("publication operation upload expiresAt must not be empty")
	}
	return nil
}

func ensureMetadata(configMap *corev1.ConfigMap) {
	if configMap.Annotations == nil {
		configMap.Annotations = map[string]string{}
	}
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	if configMap.Labels == nil {
		configMap.Labels = map[string]string{}
	}
	configMap.Labels[managedLabelKey] = managedLabelValue
}

func prepareConfigMapMutation(configMap *corev1.ConfigMap) error {
	if configMap == nil {
		return errors.New("publication operation configmap must not be nil")
	}

	ensureMetadata(configMap)
	return nil
}

func setPhaseAnnotations(configMap *corev1.ConfigMap, phase publicationports.Phase, workerName, message string) {
	configMap.Annotations[phaseAnnotationKey] = string(phase)
	setOptionalAnnotation(configMap.Annotations, workerAnnotationKey, workerName)
	setOptionalAnnotation(configMap.Annotations, messageAnnotationKey, message)
}

func setOptionalAnnotation(annotations map[string]string, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		delete(annotations, key)
		return
	}
	annotations[key] = value
}

func requiredConfigMapData(configMap *corev1.ConfigMap, key, name string) (string, error) {
	if configMap == nil {
		return "", errors.New("publication operation configmap must not be nil")
	}

	raw := optionalConfigMapData(configMap, key)
	if raw == "" {
		return "", fmt.Errorf("publication operation %s payload must not be empty", name)
	}

	return raw, nil
}

func optionalConfigMapData(configMap *corev1.ConfigMap, key string) string {
	if configMap == nil {
		return ""
	}

	return strings.TrimSpace(configMap.Data[key])
}

func decodeRequiredConfigMapJSON[T any](
	configMap *corev1.ConfigMap,
	key, name string,
	validate func(*T) error,
) (T, error) {
	raw, err := requiredConfigMapData(configMap, key, name)
	if err != nil {
		var zero T
		return zero, err
	}

	return decodeConfigMapJSON(raw, validate)
}

func decodeOptionalConfigMapJSON[T any](
	configMap *corev1.ConfigMap,
	key string,
	validate func(*T) error,
) (*T, error) {
	raw := optionalConfigMapData(configMap, key)
	if raw == "" {
		return nil, nil
	}

	value, err := decodeConfigMapJSON(raw, validate)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

func decodeConfigMapJSON[T any](raw string, validate func(*T) error) (T, error) {
	var value T
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return value, err
	}
	if validate != nil {
		if err := validate(&value); err != nil {
			var zero T
			return zero, err
		}
	}

	return value, nil
}

func setConfigMapJSON(configMap *corev1.ConfigMap, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	configMap.Data[key] = string(payload)
	return nil
}

func deleteConfigMapData(configMap *corev1.ConfigMap, keys ...string) {
	if configMap == nil || configMap.Data == nil {
		return
	}
	for _, key := range keys {
		delete(configMap.Data, key)
	}
}
