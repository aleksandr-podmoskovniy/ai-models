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

package publishworker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWorkspaceCreatesAndCleansSubdirectoryUnderSnapshotRoot(t *testing.T) {
	t.Parallel()

	snapshotRoot := t.TempDir()
	workspace, cleanupDir, err := ensureWorkspace(snapshotRoot, "ai-model-publish-")
	if err != nil {
		t.Fatalf("ensureWorkspace() error = %v", err)
	}
	if got, want := filepath.Dir(workspace), snapshotRoot; got != want {
		t.Fatalf("unexpected workspace parent %q", got)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("Stat(workspace) error = %v", err)
	}

	cleanupDir()

	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("expected workspace cleanup, got err=%v", err)
	}
	if _, err := os.Stat(snapshotRoot); err != nil {
		t.Fatalf("expected snapshot root to remain, got err=%v", err)
	}
}
