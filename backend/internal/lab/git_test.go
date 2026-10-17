package lab

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jphacks/sd_2601/internal/network"
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

func TestGitBackendUsesExplicitProxyAndReportsSourceHost(t *testing.T) {
	source := testResolvedLab().Source
	config := network.Config{HTTPSProxy: "https://proxy.example:8443"}
	environment, err := config.Environment([]string{"PATH=/bin", "HTTPS_PROXY=http://inherited.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingCommandRunner{revision: source.Revision, err: errors.New("connection timeout")}
	backend := execGitBackend{runner: runner, environment: environment}
	_, err = backend.Fetch(context.Background(), source)
	var connection *network.ConnectionError
	if !errors.As(err, &connection) || connection.Host != "github.com" || !strings.Contains(connection.Error(), "プロキシ設定") {
		t.Fatalf("fetch error = %v", err)
	}
	joined := strings.Join(runner.environments[2], "\n")
	if !strings.Contains(joined, "HTTPS_PROXY=https://proxy.example:8443") || strings.Contains(joined, "inherited.invalid") {
		t.Fatalf("unexpected Git environment: %s", joined)
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
	revision     string
	calls        [][]string
	environments [][]string
	err          error
}

func (runner *recordingCommandRunner) Run(_ context.Context, _ string, _ int64, environment []string, arguments ...string) ([]byte, error) {
	runner.calls = append(runner.calls, slices.Clone(arguments))
	runner.environments = append(runner.environments, slices.Clone(environment))
	if slices.Contains(arguments, "fetch") && runner.err != nil {
		return nil, runner.err
	}
	if slices.Contains(arguments, "rev-parse") {
		return []byte(runner.revision + "\n"), nil
	}
	if len(arguments) == 0 {
		return nil, fmt.Errorf("missing git arguments")
	}
	return nil, nil
}
