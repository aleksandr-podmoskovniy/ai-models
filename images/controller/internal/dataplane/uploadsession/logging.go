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
	"log/slog"
	"strings"
)

func sessionLogger(session SessionRecord) *slog.Logger {
	logger := slog.Default().With(
		slog.String("runtimeKind", "upload-gateway"),
		slog.String("sourceType", "Upload"),
		slog.String("sessionID", strings.TrimSpace(session.SessionID)),
		slog.String("kind", strings.TrimSpace(session.OwnerKind)),
		slog.String("name", strings.TrimSpace(session.OwnerName)),
		slog.String("phase", strings.TrimSpace(string(session.Phase))),
	)
	if namespace := strings.TrimSpace(session.OwnerNamespace); namespace != "" {
		logger = logger.With(slog.String("namespace", namespace))
	}
	if declaredFormat := strings.TrimSpace(string(session.DeclaredInputFormat)); declaredFormat != "" {
		logger = logger.With(slog.String("declaredInputFormat", declaredFormat))
	}
	return logger
}
