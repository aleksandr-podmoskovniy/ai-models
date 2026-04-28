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

package storageaccounting

import (
	"errors"
	"strings"
)

const (
	DefaultSecretName = "ai-models-storage-accounting"
	appName           = "ai-models-storage-accounting"
	ledgerDataKey     = "ledger.json"
)

type Options struct {
	Namespace  string
	SecretName string
	LimitBytes int64
}

func (o Options) Enabled() bool {
	return o.LimitBytes > 0
}

func (o Options) Normalize() Options {
	o.Namespace = strings.TrimSpace(o.Namespace)
	o.SecretName = strings.TrimSpace(o.SecretName)
	if o.SecretName == "" {
		o.SecretName = DefaultSecretName
	}
	return o
}

func (o Options) Validate() error {
	o = o.Normalize()
	if !o.Enabled() {
		return nil
	}
	switch {
	case o.Namespace == "":
		return errors.New("storage accounting namespace must not be empty")
	case o.SecretName == "":
		return errors.New("storage accounting secret name must not be empty")
	default:
		return nil
	}
}
