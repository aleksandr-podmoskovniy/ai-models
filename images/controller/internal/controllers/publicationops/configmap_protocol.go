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

package publicationops

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
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
	raw, err := requiredConfigMapData(configMap, requestDataKey, "request")
	if err != nil {
		return publicationports.Request{}, err
	}

	var request publicationports.Request
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		return publicationports.Request{}, err
	}
	if err := request.Validate(); err != nil {
		return publicationports.Request{}, err
	}
	return request, nil
}

func ResultFromConfigMap(configMap *corev1.ConfigMap) (publicationports.Result, error) {
	raw, err := requiredConfigMapData(configMap, resultDataKey, "result")
	if err != nil {
		return publicationports.Result{}, err
	}

	var result publicationports.Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return publicationports.Result{}, err
	}
	if err := result.Validate(); err != nil {
		return publicationports.Result{}, err
	}
	return result, nil
}

func WorkerResultFromConfigMap(configMap *corev1.ConfigMap) string {
	return optionalConfigMapData(configMap, workerResultDataKey)
}

func WorkerFailureFromConfigMap(configMap *corev1.ConfigMap) string {
	return optionalConfigMapData(configMap, workerFailureDataKey)
}

func UploadStatusFromConfigMap(configMap *corev1.ConfigMap) (*modelsv1alpha1.ModelUploadStatus, error) {
	raw := optionalConfigMapData(configMap, uploadDataKey)
	if raw == "" {
		return nil, nil
	}

	var status modelsv1alpha1.ModelUploadStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return nil, err
	}
	if err := validateUploadStatus(status); err != nil {
		return nil, err
	}
	return &status, nil
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

func operationStatusView(status publicationports.Status) publicationdomain.OperationStatusView {
	return publicationdomain.OperationStatusView{
		Phase:      publicationdomain.OperationPhase(status.Phase),
		WorkerName: status.WorkerName,
	}
}

func validatePersistedStatus(operation *corev1.ConfigMap, status publicationports.Status) error {
	switch status.Phase {
	case publicationports.PhasePending, publicationports.PhaseFailed:
		return nil
	case publicationports.PhaseRunning:
		if _, err := UploadStatusFromConfigMap(operation); err != nil {
			return fmt.Errorf("publication operation running state has invalid persisted upload payload: %w", err)
		}
		return nil
	case publicationports.PhaseSucceeded:
		if _, err := ResultFromConfigMap(operation); err != nil {
			return fmt.Errorf("publication operation succeeded without a valid persisted result: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("publication operation has unsupported persisted phase %q", status.Phase)
	}
}

func SetRunning(configMap *corev1.ConfigMap, workerName string) error {
	if configMap == nil {
		return errors.New("publication operation configmap must not be nil")
	}
	if strings.TrimSpace(workerName) == "" {
		return errors.New("publication operation worker name must not be empty")
	}

	ensureMetadata(configMap)
	configMap.Annotations[phaseAnnotationKey] = string(publicationports.PhaseRunning)
	configMap.Annotations[workerAnnotationKey] = workerName
	delete(configMap.Annotations, messageAnnotationKey)
	return nil
}

func SetUploadReady(configMap *corev1.ConfigMap, upload modelsv1alpha1.ModelUploadStatus) error {
	if configMap == nil {
		return errors.New("publication operation configmap must not be nil")
	}
	if err := validateUploadStatus(upload); err != nil {
		return err
	}

	payload, err := marshalConfigMapJSON(upload)
	if err != nil {
		return err
	}

	ensureMetadata(configMap)
	configMap.Data[uploadDataKey] = string(payload)
	return nil
}

func SetFailed(configMap *corev1.ConfigMap, message string) error {
	if configMap == nil {
		return errors.New("publication operation configmap must not be nil")
	}

	ensureMetadata(configMap)
	configMap.Annotations[phaseAnnotationKey] = string(publicationports.PhaseFailed)
	configMap.Annotations[messageAnnotationKey] = strings.TrimSpace(message)
	deleteConfigMapData(configMap, uploadDataKey, resultDataKey)
	return nil
}

func SetSucceeded(configMap *corev1.ConfigMap, result publicationports.Result) error {
	if configMap == nil {
		return errors.New("publication operation configmap must not be nil")
	}
	if err := result.Validate(); err != nil {
		return err
	}

	payload, err := marshalConfigMapJSON(result)
	if err != nil {
		return err
	}

	ensureMetadata(configMap)
	configMap.Annotations[phaseAnnotationKey] = string(publicationports.PhaseSucceeded)
	delete(configMap.Annotations, messageAnnotationKey)
	deleteConfigMapData(configMap, uploadDataKey)
	configMap.Data[resultDataKey] = string(payload)
	return nil
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

func marshalConfigMapJSON(value any) ([]byte, error) {
	return json.Marshal(value)
}

func deleteConfigMapData(configMap *corev1.ConfigMap, keys ...string) {
	if configMap == nil || configMap.Data == nil {
		return
	}
	for _, key := range keys {
		delete(configMap.Data, key)
	}
}
