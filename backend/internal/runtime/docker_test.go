package runtime

import (
	"context"
	"slices"
	"testing"
)

func TestDockerComposeProviderUsesRunnerOwnedProjectAndTemporaryCompose(t *testing.T) {
	runner := &recordingDockerRunner{}
	provider := newDockerComposeProviderForTest(runner)
	project := "reprokit_0123456789ab"
	if err := provider.Start(context.Background(), project, []byte("services: {}\n")); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	call := runner.calls[0]
	if !slices.Contains(call, "compose") || !slices.Contains(call, "--project-name") || !slices.Contains(call, project) || !slices.Contains(call, "--detach") {
		t.Fatalf("unexpected command: %#v", call)
	}
	if err := provider.Start(context.Background(), "outside_scope", []byte("services: {}\n")); err == nil {
		t.Fatal("unsafe project name was accepted")
	}
}

func TestDockerComposeProviderInspectsProjectLabels(t *testing.T) {
	runner := &recordingDockerRunner{outputs: [][]byte{
		[]byte(`{"Names":"reprokit_0123456789ab-app-1","State":"running"}` + "\n"),
		[]byte("reprokit_0123456789ab_default\n"),
		[]byte("reprokit_0123456789ab_data\n"),
	}}
	provider := newDockerComposeProviderForTest(runner)
	resources, err := provider.Inspect(context.Background(), "reprokit_0123456789ab", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Containers) != 1 || resources.Containers[0].State != "running" || len(resources.Networks) != 1 || len(resources.Volumes) != 1 {
		t.Fatalf("resources = %#v", resources)
	}
	for _, call := range runner.calls {
		if !slices.Contains(call, "label=com.docker.compose.project=reprokit_0123456789ab") {
			t.Fatalf("missing project label filter: %#v", call)
		}
	}
}

type recordingDockerRunner struct {
	calls   [][]string
	outputs [][]byte
}

func (runner *recordingDockerRunner) Run(_ context.Context, arguments ...string) ([]byte, error) {
	runner.calls = append(runner.calls, slices.Clone(arguments))
	if len(runner.outputs) == 0 {
		return nil, nil
	}
	output := runner.outputs[0]
	runner.outputs = runner.outputs[1:]
	return output, nil
}
