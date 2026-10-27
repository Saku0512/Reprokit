package lab

import (
	"context"
	"slices"
	"strings"
	"testing"
)

type recordingCommandRunner struct {
	revision string
	calls    [][]string
}

func (runner *recordingCommandRunner) Run(_ context.Context, _ string, _ int64, _ []string, arguments ...string) ([]byte, error) {
	runner.calls = append(runner.calls, slices.Clone(arguments))
	return []byte(runner.revision + "\n"), nil
}

func TestLocalGitBackendUsesOnlyFixedLocalRevision(t *testing.T) {
	source := Source{Type: "git", URL: "https://github.com/Saku0512/Reprokit.git", Revision: strings.Repeat("a", 40), Subdirectory: "images/CVE-2026-54088"}
	runner := &recordingCommandRunner{revision: source.Revision}
	backend := localGitBackend{repositoryPath: "/not-used", runner: runner}
	snapshot, err := backend.Fetch(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if len(runner.calls) != 1 || !slices.Equal(runner.calls[0], []string{"rev-parse", "--verify", source.Revision + "^{commit}"}) {
		t.Fatalf("unexpected Git calls: %#v", runner.calls)
	}
	runner.revision = strings.Repeat("b", 40)
	if _, err := backend.Fetch(context.Background(), source); err == nil {
		t.Fatal("revision mismatch was accepted")
	}
}
