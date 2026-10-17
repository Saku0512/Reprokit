package lab

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestGitBackendFetchesAndVerifiesExactRevision(t *testing.T) {
	source := testResolvedLab().Source
	runner := &recordingCommandRunner{revision: source.Revision}
	backend := execGitBackend{runner: runner}

	snapshot, err := backend.Fetch(context.Background(), source)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	defer snapshot.Close()

	var fetchCall []string
	for _, call := range runner.calls {
		if slices.Contains(call, "fetch") {
			fetchCall = call
		}
	}
	if fetchCall == nil {
		t.Fatalf("git fetch was not called: %#v", runner.calls)
	}
	wantTail := []string{"fetch", "--depth=1", "--no-tags", "origin", source.Revision}
	if len(fetchCall) < len(wantTail) || !slices.Equal(fetchCall[len(fetchCall)-len(wantTail):], wantTail) {
		t.Fatalf("fetch call = %#v, want suffix %#v", fetchCall, wantTail)
	}
	if slices.Contains(fetchCall, "master") || slices.Contains(fetchCall, "main") {
		t.Fatalf("fetch call follows a mutable branch: %#v", fetchCall)
	}
}

func TestGitBackendRejectsRevisionMismatch(t *testing.T) {
	source := testResolvedLab().Source
	runner := &recordingCommandRunner{revision: strings.Repeat("f", 40)}
	backend := execGitBackend{runner: runner}

	_, err := backend.Fetch(context.Background(), source)
	if err == nil || !strings.Contains(err.Error(), "does not match requested revision") {
		t.Fatalf("Fetch() error = %v, want revision mismatch", err)
	}
}

type recordingCommandRunner struct {
	revision string
	calls    [][]string
}

func (runner *recordingCommandRunner) Run(_ context.Context, _ string, _ int64, arguments ...string) ([]byte, error) {
	runner.calls = append(runner.calls, slices.Clone(arguments))
	if slices.Contains(arguments, "rev-parse") {
		return []byte(runner.revision + "\n"), nil
	}
	if len(arguments) == 0 {
		return nil, fmt.Errorf("missing git arguments")
	}
	return nil, nil
}
