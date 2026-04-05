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
	"errors"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsession"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	PublishPod   PublishPodOptions
	RequeueAfter time.Duration
}

type PublishPodOptions struct {
	Namespace               string
	Image                   string
	ServiceAccountName      string
	OCIRepositoryPrefix     string
	OCIInsecure             bool
	OCIRegistrySecretName   string
	OCIRegistryCASecretName string
}

type Reconciler struct {
	client         client.Client
	options        Options
	sourceWorkers  publicationports.SourceWorkerRuntime
	uploadSessions publicationports.UploadSessionRuntime
}

func SetupWithManager(mgr ctrl.Manager, options Options) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}
	if !options.Enabled() {
		return nil
	}

	options = normalizeOptions(options)
	if err := options.Validate(); err != nil {
		return err
	}

	reconciler, err := newReconciler(mgr.GetClient(), mgr.GetScheme(), options)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Complete(reconciler)
}

func (o Options) Enabled() bool {
	return strings.TrimSpace(o.PublishPod.Namespace) != "" &&
		strings.TrimSpace(o.PublishPod.Image) != ""
}

func (o Options) Validate() error {
	if !o.Enabled() {
		return nil
	}
	return o.PublishPod.sourceWorkerOptions().Validate()
}

func normalizeOptions(options Options) Options {
	if options.RequeueAfter <= 0 {
		options.RequeueAfter = 5 * time.Second
	}

	return options
}

func (o PublishPodOptions) sourceWorkerOptions() sourceworker.Options {
	return sourceworker.Options{
		Namespace:               o.Namespace,
		Image:                   o.Image,
		ServiceAccountName:      o.ServiceAccountName,
		OCIRepositoryPrefix:     o.OCIRepositoryPrefix,
		OCIInsecure:             o.OCIInsecure,
		OCIRegistrySecretName:   o.OCIRegistrySecretName,
		OCIRegistryCASecretName: o.OCIRegistryCASecretName,
	}
}

func (o PublishPodOptions) uploadSessionOptions() uploadsession.Options {
	return uploadsession.Options{
		Namespace:               o.Namespace,
		Image:                   o.Image,
		ServiceAccountName:      o.ServiceAccountName,
		OCIRepositoryPrefix:     o.OCIRepositoryPrefix,
		OCIInsecure:             o.OCIInsecure,
		OCIRegistrySecretName:   o.OCIRegistrySecretName,
		OCIRegistryCASecretName: o.OCIRegistryCASecretName,
	}
}
