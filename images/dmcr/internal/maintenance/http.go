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

package maintenance

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/distribution/distribution/v3/registry/api/errcode"
)

func RegistryWriteGateHandler(checker Checker, next http.Handler) http.Handler {
	if checker == nil {
		return next
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if isRegistryWrite(request) {
			if blockedByGate(writer, request, checker) {
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}

func RejectWriteIfActive(writer http.ResponseWriter, request *http.Request, checker Checker) bool {
	if checker == nil {
		return false
	}
	return blockedByGate(writer, request, checker)
}

func blockedByGate(writer http.ResponseWriter, request *http.Request, checker Checker) bool {
	active, err := checker.Active(request.Context())
	if err != nil {
		writeUnavailable(writer, "dmcr maintenance gate check failed")
		slog.Default().Warn("dmcr maintenance gate check failed", slog.Any("error", err))
		return true
	}
	if !active {
		return false
	}
	writeUnavailable(writer, "dmcr is temporarily read-only for garbage collection")
	slog.Default().Info(
		"dmcr write rejected during garbage collection",
		slog.String("method", request.Method),
		slog.String("path", request.URL.Path),
	)
	return true
}

func writeUnavailable(writer http.ResponseWriter, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(writer).Encode(errcode.Errors{
		errcode.ErrorCodeUnavailable.WithMessage(message),
	})
}

func isRegistryWrite(request *http.Request) bool {
	if request == nil || !strings.HasPrefix(request.URL.Path, "/v2/") {
		return false
	}
	switch request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
