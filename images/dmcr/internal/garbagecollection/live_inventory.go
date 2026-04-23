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

package garbagecollection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const cleanupHandleAnnotationKey = "ai.deckhouse.io/cleanup-handle"

var (
	modelGVR        = schema.GroupVersionResource{Group: "ai.deckhouse.io", Version: "v1alpha1", Resource: "models"}
	clusterModelGVR = schema.GroupVersionResource{Group: "ai.deckhouse.io", Version: "v1alpha1", Resource: "clustermodels"}
)

type cleanupHandleSnapshot struct {
	Kind    string                        `json:"kind,omitempty"`
	Backend *cleanupHandleBackendSnapshot `json:"backend,omitempty"`
}

type cleanupHandleBackendSnapshot struct {
	Reference                string `json:"reference,omitempty"`
	RepositoryMetadataPrefix string `json:"repositoryMetadataPrefix,omitempty"`
	SourceMirrorPrefix       string `json:"sourceMirrorPrefix,omitempty"`
}

func DiscoverLivePrefixes(ctx context.Context, client dynamic.Interface) (livePrefixSet, error) {
	if client == nil {
		return livePrefixSet{}, fmt.Errorf("dynamic client must not be nil")
	}

	live := newLivePrefixSet()
	if err := collectLivePrefixesForResource(ctx, client, modelGVR, true, &live); err != nil {
		return livePrefixSet{}, err
	}
	if err := collectLivePrefixesForResource(ctx, client, clusterModelGVR, false, &live); err != nil {
		return livePrefixSet{}, err
	}
	return live, nil
}

func collectLivePrefixesForResource(
	ctx context.Context,
	client dynamic.Interface,
	gvr schema.GroupVersionResource,
	namespaced bool,
	live *livePrefixSet,
) error {
	resource := client.Resource(gvr)
	var (
		list *unstructured.UnstructuredList
		err  error
	)

	if namespaced {
		list, err = resource.Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = resource.List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return fmt.Errorf("list %s: %w", gvr.Resource, err)
	}

	for _, item := range list.Items {
		if namespaced {
			live.modelCount++
		} else {
			live.clusterModelCount++
		}
		if err := collectLivePrefixesFromObject(item.GetNamespace(), item.GetName(), item.GetAnnotations(), live); err != nil {
			return err
		}
	}
	return nil
}

func collectLivePrefixesFromObject(
	namespace string,
	name string,
	annotations map[string]string,
	live *livePrefixSet,
) error {
	if live == nil {
		return fmt.Errorf("live prefix set must not be nil")
	}

	rawHandle := strings.TrimSpace(annotations[cleanupHandleAnnotationKey])
	if rawHandle == "" {
		return nil
	}

	var handle cleanupHandleSnapshot
	if err := json.Unmarshal([]byte(rawHandle), &handle); err != nil {
		return fmt.Errorf("decode cleanup handle for %s: %w", objectRef(namespace, name), err)
	}

	if strings.TrimSpace(handle.Kind) != "BackendArtifact" || handle.Backend == nil {
		return nil
	}

	if prefix := strings.Trim(strings.TrimSpace(handle.Backend.RepositoryMetadataPrefix), "/"); prefix != "" {
		live.addRepository(prefix)
	} else if prefix := repositoryMetadataPrefixFromReference(handle.Backend.Reference); prefix != "" {
		live.addRepository(prefix)
	}
	if prefix := strings.Trim(strings.TrimSpace(handle.Backend.SourceMirrorPrefix), "/"); prefix != "" {
		live.addRaw(prefix)
	}
	return nil
}

func objectRef(namespace, name string) string {
	if strings.TrimSpace(namespace) == "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(namespace) + "/" + strings.TrimSpace(name)
}
