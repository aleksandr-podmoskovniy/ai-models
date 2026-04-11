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
	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) recordAudit(object client.Object, records []auditsink.Record) {
	if r == nil || r.auditSink == nil || object == nil {
		return
	}
	for _, record := range records {
		r.auditSink.Record(object, record)
	}
}
