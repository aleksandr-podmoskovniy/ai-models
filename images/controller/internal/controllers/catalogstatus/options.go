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

package catalogstatus

import (
	"errors"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	OperationNamespace string
}

type baseReconciler struct {
	client  client.Client
	options Options
}

type ModelReconciler struct{ baseReconciler }
type ClusterModelReconciler struct{ baseReconciler }

func SetupWithManager(mgr ctrl.Manager, options Options) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}
	if !options.Enabled() {
		return nil
	}

	if err := options.Validate(); err != nil {
		return err
	}

	base := baseReconciler{
		client:  mgr.GetClient(),
		options: options,
	}

	if err := ctrl.NewControllerManagedBy(mgr).For(&modelsv1alpha1.Model{}).Complete(&ModelReconciler{base}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).For(&modelsv1alpha1.ClusterModel{}).Complete(&ClusterModelReconciler{base})
}

func (o Options) Enabled() bool {
	return strings.TrimSpace(o.OperationNamespace) != ""
}

func (o Options) Validate() error {
	if !o.Enabled() {
		return nil
	}
	if strings.TrimSpace(o.OperationNamespace) == "" {
		return errors.New("publication operation namespace must not be empty")
	}

	return nil
}
