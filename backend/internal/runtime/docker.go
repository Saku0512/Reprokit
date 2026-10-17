package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var projectPattern = regexp.MustCompile(`^reprokit_[0-9a-f]{12}$`)

type dockerCommandRunner interface {
	Run(context.Context, ...string) ([]byte, error)
}

type execDockerCommandRunner struct{}

func (execDockerCommandRunner) Run(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	var output limitedOutput
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("docker %s: %w: %s", arguments[0], err, strings.TrimSpace(output.String()))
	}
	return output.Bytes(), nil
}

type limitedOutput struct {
	bytes.Buffer
}

func (output *limitedOutput) Write(contents []byte) (int, error) {
	const limit = 1 << 20
	remaining := limit - output.Len()
	if remaining <= 0 {
		return 0, errors.New("docker output exceeds limit")
	}
	if len(contents) > remaining {
		written, _ := output.Buffer.Write(contents[:remaining])
		return written, errors.New("docker output exceeds limit")
	}
	return output.Buffer.Write(contents)
}

// DockerComposeProvider はRunnerが作成した一時ComposeファイルだけをDockerへ渡す。
type DockerComposeProvider struct {
	runner dockerCommandRunner
}

func NewDockerComposeProvider() *DockerComposeProvider {
	return &DockerComposeProvider{runner: execDockerCommandRunner{}}
}

func newDockerComposeProviderForTest(runner dockerCommandRunner) *DockerComposeProvider {
	return &DockerComposeProvider{runner: runner}
}

func (provider *DockerComposeProvider) Start(ctx context.Context, project string, compose []byte) error {
	return provider.withCompose(ctx, project, compose, "up", "--detach", "--remove-orphans")
}

func (provider *DockerComposeProvider) Stop(ctx context.Context, project string, compose []byte) error {
	return provider.withCompose(ctx, project, compose, "stop")
}

func (provider *DockerComposeProvider) Delete(ctx context.Context, project string, compose []byte, removeVolumes bool) error {
	arguments := []string{"down", "--remove-orphans"}
	if removeVolumes {
		arguments = append(arguments, "--volumes")
	}
	return provider.withCompose(ctx, project, compose, arguments...)
}

func (provider *DockerComposeProvider) Inspect(ctx context.Context, project string, _ []byte) (Resources, error) {
	if !projectPattern.MatchString(project) {
		return Resources{}, errors.New("invalid Runner project name")
	}
	filter := "label=com.docker.compose.project=" + project
	containers, err := provider.runner.Run(ctx, "ps", "--all", "--filter", filter, "--format", "{{json .}}")
	if err != nil {
		return Resources{}, err
	}
	networks, err := provider.runner.Run(ctx, "network", "ls", "--filter", filter, "--format", "{{.Name}}")
	if err != nil {
		return Resources{}, err
	}
	volumes, err := provider.runner.Run(ctx, "volume", "ls", "--filter", filter, "--format", "{{.Name}}")
	if err != nil {
		return Resources{}, err
	}
	return Resources{
		Containers: decodeContainers(containers),
		Networks:   decodeNames(networks),
		Volumes:    decodeNames(volumes),
	}, nil
}

func (provider *DockerComposeProvider) withCompose(ctx context.Context, project string, compose []byte, action ...string) error {
	if !projectPattern.MatchString(project) {
		return errors.New("invalid Runner project name")
	}
	directory, err := os.MkdirTemp("", "reprokit-compose-*")
	if err != nil {
		return fmt.Errorf("create Compose staging directory: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "compose.yaml")
	if err := os.WriteFile(path, compose, 0o600); err != nil {
		return fmt.Errorf("write Compose staging file: %w", err)
	}
	arguments := []string{"compose", "--project-name", project, "--file", path}
	arguments = append(arguments, action...)
	_, err = provider.runner.Run(ctx, arguments...)
	return err
}

func decodeContainers(output []byte) []Container {
	var result []Container
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		var row struct {
			Names string `json:"Names"`
			State string `json:"State"`
		}
		if err := json.Unmarshal([]byte(line), &row); err == nil && row.Names != "" {
			result = append(result, Container{Name: row.Names, State: row.State})
		}
	}
	return result
}

func decodeNames(output []byte) []string {
	var result []string
	for _, name := range strings.Split(string(output), "\n") {
		if name = strings.TrimSpace(name); name != "" {
			result = append(result, name)
		}
	}
	return result
}
