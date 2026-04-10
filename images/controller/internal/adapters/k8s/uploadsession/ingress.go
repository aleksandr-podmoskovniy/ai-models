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

package uploadsession

import (
	"context"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) ensureIngress(
	ctx context.Context,
	owner client.Object,
	ownerUID types.UID,
	token string,
) (*networkingv1.Ingress, bool, error) {
	if !s.options.Ingress.Enabled() {
		return nil, false, nil
	}
	ingress, err := s.buildIngress(ownerUID, token)
	if err != nil {
		return nil, false, err
	}
	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, ingress)
	if err != nil {
		return nil, false, err
	}
	return ingress, created, nil
}

func (s *Service) buildIngress(ownerUID types.UID, token string) (*networkingv1.Ingress, error) {
	name, err := resourcenames.UploadSessionIngressName(ownerUID)
	if err != nil {
		return nil, err
	}
	serviceName, err := resourcenames.UploadSessionServiceName(ownerUID)
	if err != nil {
		return nil, err
	}
	path := uploadSessionPath(token)
	pathType := networkingv1.PathTypeExact

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.options.Runtime.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-body-size":         "0",
				"nginx.ingress.kubernetes.io/proxy-request-buffering": "off",
				"nginx.ingress.kubernetes.io/proxy-buffering":         "off",
				"nginx.ingress.kubernetes.io/proxy-read-timeout":      "3600",
				"nginx.ingress.kubernetes.io/proxy-send-timeout":      "3600",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: s.options.Ingress.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     path,
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: serviceName,
									Port: networkingv1.ServiceBackendPort{Number: uploadPort},
								},
							},
						}},
					},
				},
			}},
		},
	}
	if s.options.Ingress.ClassName != "" {
		ingress.Spec.IngressClassName = &s.options.Ingress.ClassName
	}
	if s.options.Ingress.TLSSecretName != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{s.options.Ingress.Host},
			SecretName: s.options.Ingress.TLSSecretName,
		}}
	}
	return ingress, nil
}
