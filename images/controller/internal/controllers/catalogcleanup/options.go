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

package catalogcleanup

import (
	"errors"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Finalizer = "ai-models.deckhouse.io/model-cleanup"

const (
	modelControllerName        = "catalogcleanup-model"
	clusterModelControllerName = "catalogcleanup-cluster-model"
)

type Options struct {
	CleanupJob   CleanupJobOptions
	RequeueAfter time.Duration
}

type baseReconciler struct {
	client  client.Client
	scheme  *runtime.Scheme
	options Options
}

type ModelReconciler struct{ baseReconciler }
type ClusterModelReconciler struct{ baseReconciler }

func SetupWithManager(mgr ctrl.Manager, options Options) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}
	if err := options.CleanupJob.Validate(); err != nil {
		return err
	}
	if options.RequeueAfter <= 0 {
		options.RequeueAfter = 5 * time.Second
	}

	base := baseReconciler{
		client:  mgr.GetClient(),
		scheme:  mgr.GetScheme(),
		options: options,
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		Named(modelControllerName).
		For(&modelsv1alpha1.Model{}).
		Complete(&ModelReconciler{base}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(clusterModelControllerName).
		For(&modelsv1alpha1.ClusterModel{}).
		Complete(&ClusterModelReconciler{base})
}
