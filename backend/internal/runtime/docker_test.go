package runtime

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jphacks/sd_2601/internal/network"
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

func TestDockerComposeProviderChecksRunnerIngressIsActuallyPublished(t *testing.T) {
	runner := &recordingDockerRunner{outputs: [][]byte{[]byte("127.0.0.1:8080\n")}}
	provider := newDockerComposeProviderForTest(runner)
	compose := []byte("services:\n  reprokit_proxy_app_1:\n    image: caddy@sha256:d8c17a862962def15cde69863a3a463f25a2664942eafd7bdbf050e9c3116b83\n    ports:\n      - \"127.0.0.1:8080:80\"\n")
	if err := provider.CheckEndpoints(context.Background(), "reprokit_0123456789ab", compose); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || !slices.Contains(runner.calls[0], "port") || !slices.Contains(runner.calls[0], "reprokit_proxy_app_1") {
		t.Fatalf("unexpected Docker command: %#v", runner.calls)
	}
	runner = &recordingDockerRunner{outputs: [][]byte{[]byte("\n")}}
	provider = newDockerComposeProviderForTest(runner)
	if err := provider.CheckEndpoints(context.Background(), "reprokit_0123456789ab", compose); err == nil {
		t.Fatal("missing Docker port publication was accepted")
	}
}

func TestDockerComposeProviderUsesTemporaryCredentialsAndExplainsConnectionFailure(t *testing.T) {
	runner := &recordingDockerRunner{err: errors.New("pull timeout")}
	config := network.Config{RegistryCredentials: []network.RegistryCredential{{Registry: "registry.example", Username: "learner", Password: "secret"}}}
	provider, err := NewDockerComposeProviderWithNetwork(config)
	if err != nil {
		t.Fatal(err)
	}
	provider.runner = runner
	err = provider.Start(context.Background(), "reprokit_0123456789ab", []byte("services:\n  app:\n    image: registry.example/reprokit/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"))
	var connection *network.ConnectionError
	if !errors.As(err, &connection) || connection.Host != "registry.example" || strings.Contains(connection.Error(), "secret") {
		t.Fatalf("start error = %v", err)
	}
	call := runner.calls[0]
	index := slices.Index(call, "--config")
	if index < 0 || index+1 >= len(call) {
		t.Fatalf("temporary Docker config was not used: %#v", call)
	}
	if _, err := os.Stat(call[index+1]); !os.IsNotExist(err) {
		t.Fatalf("temporary Docker config remains after command: %v", err)
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

func TestDockerComposeProviderMeasuresOnlyProjectWritableLayers(t *testing.T) {
	runner := &recordingDockerRunner{outputs: [][]byte{
		[]byte("0123456789ab\nabcdef012345\n"),
		[]byte("1024\n"),
		[]byte("2048\n"),
	}}
	provider := newDockerComposeProviderForTest(runner)
	usage, err := provider.DiskUsage(context.Background(), "reprokit_0123456789ab")
	if err != nil || usage != 3072 {
		t.Fatalf("DiskUsage = %d, %v", usage, err)
	}
	if len(runner.calls) != 3 || !slices.Contains(runner.calls[0], "label=com.docker.compose.project=reprokit_0123456789ab") {
		t.Fatalf("unexpected Docker commands: %#v", runner.calls)
	}
}

func TestDockerComposeProviderRejectsInvalidWritableLayerSize(t *testing.T) {
	runner := &recordingDockerRunner{outputs: [][]byte{[]byte("0123456789ab\n"), []byte("unknown\n")}}
	provider := newDockerComposeProviderForTest(runner)
	if _, err := provider.DiskUsage(context.Background(), "reprokit_0123456789ab"); err == nil {
		t.Fatal("invalid writable layer size was accepted")
	}
}

func TestDockerComposeProviderReadsOnlyFixedMarker(t *testing.T) {
	runner := &recordingDockerRunner{outputs: [][]byte{[]byte("uid=1000(filebrowser)\n")}}
	provider := newDockerComposeProviderForTest(runner)
	marker, err := provider.ReadMarker(context.Background(), "reprokit_0123456789ab", []byte("services: {}\n"), "vulnerable")
	if err != nil || marker != "uid=1000(filebrowser)" {
		t.Fatalf("marker=%q err=%v", marker, err)
	}
	call := runner.calls[0]
	if !slices.Contains(call, "exec") || !slices.Contains(call, "vulnerable") || !slices.Contains(call, "/bin/sh") || !slices.Contains(call, "if [ -r /tmp/success ]; then head -c 512 /tmp/success; fi") {
		t.Fatalf("unexpected marker command: %#v", call)
	}
	if _, err := provider.ReadMarker(context.Background(), "reprokit_0123456789ab", nil, "other"); err == nil {
		t.Fatal("unknown service was accepted")
	}
}

type recordingDockerRunner struct {
	calls   [][]string
	outputs [][]byte
	err     error
}

func (runner *recordingDockerRunner) Run(_ context.Context, _ []string, arguments ...string) ([]byte, error) {
	runner.calls = append(runner.calls, slices.Clone(arguments))
	if runner.err != nil {
		return nil, runner.err
	}
	if len(runner.outputs) == 0 {
		return nil, nil
	}
	output := runner.outputs[0]
	runner.outputs = runner.outputs[1:]
	return output, nil
}
