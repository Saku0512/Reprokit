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
	"slices"
	"strconv"
	"strings"

	"github.com/jphacks/sd_2601/internal/network"
	"gopkg.in/yaml.v3"
)

var projectPattern = regexp.MustCompile(`^reprokit_[0-9a-f]{12}$`)
var containerIDPattern = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

const (
	logTailLines   = 200
	maxLogBytes    = 64 * 1024
	maxMarkerBytes = 512
)

type dockerCommandRunner interface {
	Run(context.Context, []string, ...string) ([]byte, error)
}

type execDockerCommandRunner struct{}

func (execDockerCommandRunner) Run(ctx context.Context, environment []string, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	command.Env = environment
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
	runner  dockerCommandRunner
	network network.Config
	env     []string
}

// PortConflictErrorはLabのloopback公開ポートが使用中の場合だけ、復旧方法を固定して返す。
type PortConflictError struct {
	Port  int
	cause error
}

func (err *PortConflictError) Error() string {
	if err.Port > 0 {
		return fmt.Sprintf("公開ポート 127.0.0.1:%d は他のプロセスが使用中です。Labの起動を中止しました。使用中のアプリを安全に停止するか、ポート競合のない環境で再試行してください。", err.Port)
	}
	return "Labの公開ポートは他のプロセスが使用中です。Labの起動を中止しました。使用中のアプリを安全に停止するか、ポート競合のない環境で再試行してください。"
}

func (err *PortConflictError) Unwrap() error { return err.cause }

func NewDockerComposeProvider() *DockerComposeProvider {
	provider, err := NewDockerComposeProviderWithNetwork(network.Config{})
	if err != nil {
		panic(err)
	}
	return provider
}

// NewDockerComposeProviderWithNetwork は認証情報を一時Docker設定としてだけ利用する。
func NewDockerComposeProviderWithNetwork(config network.Config) (*DockerComposeProvider, error) {
	environment, err := config.Environment(os.Environ())
	if err != nil {
		return nil, err
	}
	return &DockerComposeProvider{runner: execDockerCommandRunner{}, network: config, env: environment}, nil
}

func newDockerComposeProviderForTest(runner dockerCommandRunner) *DockerComposeProvider {
	return &DockerComposeProvider{runner: runner, env: []string{"PATH=/usr/bin"}}
}

func (provider *DockerComposeProvider) Start(ctx context.Context, project string, compose []byte) error {
	_, err := provider.withCompose(ctx, project, compose, "up", "--detach", "--remove-orphans")
	return err
}

// CheckEndpoints はComposeが返す実際の公開先を確認する。
// Docker Engineがinternal Bridge上の公開指定を黙って無視する場合を起動失敗として扱う。
func (provider *DockerComposeProvider) CheckEndpoints(ctx context.Context, project string, compose []byte) error {
	for _, endpoint := range ingressEndpoints(compose) {
		output, err := provider.withCompose(ctx, project, compose, "port", endpoint.service, "80")
		if err != nil {
			return fmt.Errorf("verify loopback endpoint %s: %w", endpoint.service, err)
		}
		actual := strings.TrimSpace(string(output))
		expected := "127.0.0.1:" + strconv.Itoa(endpoint.port)
		if actual != expected {
			return fmt.Errorf("verify loopback endpoint %s: got %q, want %q", endpoint.service, actual, expected)
		}
	}
	return nil
}

func (provider *DockerComposeProvider) Stop(ctx context.Context, project string, compose []byte) error {
	_, err := provider.withCompose(ctx, project, compose, "stop")
	return err
}

func (provider *DockerComposeProvider) Delete(ctx context.Context, project string, compose []byte, removeVolumes bool) error {
	arguments := []string{"down", "--remove-orphans"}
	if removeVolumes {
		arguments = append(arguments, "--volumes")
	}
	_, err := provider.withCompose(ctx, project, compose, arguments...)
	return err
}

// ReadMarker は比較Lab内の固定/tmp/successだけを最大512 byte読取る。
// service、パス、実行ファイルと引数は固定であり、HTTP入力をDocker実行へ転用しない。
func (provider *DockerComposeProvider) ReadMarker(ctx context.Context, project string, compose []byte, service string) (string, error) {
	if service != "vulnerable" && service != "fixed" {
		return "", errors.New("unknown File Browser comparison service")
	}
	marker, err := provider.withCompose(ctx, project, compose, "exec", "--no-TTY", service, "find", "/tmp", "-maxdepth", "1", "-type", "f", "-name", "success", "-print")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(marker)) != "/tmp/success" {
		return "", nil
	}
	output, err := provider.withCompose(ctx, project, compose, "exec", "--no-TTY", service, "cat", "/tmp/success")
	if err != nil {
		return "", err
	}
	if len(output) > maxMarkerBytes {
		output = output[:maxMarkerBytes]
	}
	return strings.TrimSpace(string(output)), nil
}

// ReadLogs は固定された比較対象サービスの直近ログだけを追従せずに読む。
// コンテナ名やDockerオプションはHTTP入力から受け取らず、入口プロキシも対象にしない。
func (provider *DockerComposeProvider) ReadLogs(ctx context.Context, project string, compose []byte, service string) (LogSnapshot, error) {
	if service != "vulnerable" && service != "fixed" {
		return LogSnapshot{}, errors.New("unknown File Browser comparison service")
	}
	output, err := provider.withCompose(ctx, project, compose, "logs", "--no-color", "--no-log-prefix", "--tail", strconv.Itoa(logTailLines), service)
	if err != nil {
		return LogSnapshot{}, err
	}
	return truncateLogs(output, service), nil
}

func truncateLogs(output []byte, service string) LogSnapshot {
	content, truncated := truncateOutput(output)
	return LogSnapshot{Service: service, Content: content, Truncated: truncated}
}

// RunAction はFile Browser比較Labでレビュー済みの観察だけを引数配列で実行する。
// シェルや利用者指定のコマンド・引数・パスは使わない。
func (provider *DockerComposeProvider) RunAction(ctx context.Context, project string, compose []byte, service string, action ObservationAction) (ActionObservation, error) {
	if service != "vulnerable" && service != "fixed" {
		return ActionObservation{}, errors.New("unknown File Browser comparison service")
	}
	if action != ListTmpAction {
		return ActionObservation{}, errors.New("unknown File Browser observation action")
	}
	output, err := provider.withCompose(ctx, project, compose, "exec", "--no-TTY", service, "ls", "-la", "/tmp")
	if err != nil {
		return ActionObservation{}, err
	}
	content, truncated := truncateOutput(output)
	return ActionObservation{Service: service, Action: action, Output: content, Truncated: truncated}, nil
}

func truncateOutput(output []byte) (string, bool) {
	if len(output) <= maxLogBytes {
		return string(output), false
	}
	prefix := "[ログの先頭を省略しました。直近64 KiBを表示しています。]\n"
	limit := maxLogBytes - len(prefix)
	return prefix + string(output[len(output)-limit:]), true
}

func (provider *DockerComposeProvider) Inspect(ctx context.Context, project string, _ []byte) (Resources, error) {
	if !projectPattern.MatchString(project) {
		return Resources{}, errors.New("invalid Runner project name")
	}
	filter := "label=com.docker.compose.project=" + project
	containers, err := provider.runner.Run(ctx, provider.env, "ps", "--all", "--filter", filter, "--format", "{{json .}}")
	if err != nil {
		return Resources{}, err
	}
	networks, err := provider.runner.Run(ctx, provider.env, "network", "ls", "--filter", filter, "--format", "{{.Name}}")
	if err != nil {
		return Resources{}, err
	}
	volumes, err := provider.runner.Run(ctx, provider.env, "volume", "ls", "--filter", filter, "--format", "{{.Name}}")
	if err != nil {
		return Resources{}, err
	}
	return Resources{
		Containers: decodeContainers(containers),
		Networks:   decodeNames(networks),
		Volumes:    decodeNames(volumes),
	}, nil
}

// DiskUsage はRunner自身のCompose Projectに属するコンテナの書込み量だけを合計する。
func (provider *DockerComposeProvider) DiskUsage(ctx context.Context, project string) (int64, error) {
	if !projectPattern.MatchString(project) {
		return 0, errors.New("invalid Runner project name")
	}
	filter := "label=com.docker.compose.project=" + project
	containers, err := provider.runner.Run(ctx, provider.env, "ps", "--all", "--filter", filter, "--format", "{{.ID}}")
	if err != nil {
		return 0, err
	}
	var total int64
	for _, id := range strings.Fields(string(containers)) {
		if !containerIDPattern.MatchString(id) {
			return 0, errors.New("invalid Docker container ID")
		}
		output, err := provider.runner.Run(ctx, provider.env, "inspect", "--size", "--format", "{{.SizeRw}}", id)
		if err != nil {
			return 0, err
		}
		usage, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		if err != nil || usage < 0 || total > (1<<63-1)-usage {
			return 0, errors.New("invalid Docker writable layer size")
		}
		total += usage
	}
	return total, nil
}

func (provider *DockerComposeProvider) withCompose(ctx context.Context, project string, compose []byte, action ...string) ([]byte, error) {
	if !projectPattern.MatchString(project) {
		return nil, errors.New("invalid Runner project name")
	}
	directory, err := os.MkdirTemp("", "reprokit-compose-*")
	if err != nil {
		return nil, fmt.Errorf("create Compose staging directory: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "compose.yaml")
	if err := os.WriteFile(path, compose, 0o600); err != nil {
		return nil, fmt.Errorf("write Compose staging file: %w", err)
	}
	arguments := []string{}
	configDirectory, cleanup, err := provider.network.DockerConfigDir()
	if err != nil {
		return nil, fmt.Errorf("create temporary registry credentials: %w", err)
	}
	defer cleanup()
	if configDirectory != "" {
		arguments = append(arguments, "--config", configDirectory)
	}
	arguments = append(arguments, "compose", "--project-name", project, "--file", path)
	arguments = append(arguments, action...)
	output, err := provider.runner.Run(ctx, provider.env, arguments...)
	if err != nil && len(action) > 0 && action[0] == "up" {
		if conflict := portConflict(err, compose); conflict != nil {
			return nil, conflict
		}
		if likelyConnectionFailure(err) {
			return nil, network.NewConnectionError("コンテナイメージの取得", strings.Join(registryHosts(compose), ", "), "プロキシ設定、レジストリ認証情報、またはネットワーク接続を確認してください", err)
		}
	}
	return output, err
}

func portConflict(err error, compose []byte) error {
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "port is already allocated") && !strings.Contains(message, "address already in use") {
		return nil
	}
	ports := publishedPorts(compose)
	for _, port := range ports {
		if errorMentionsPort(message, port) {
			return &PortConflictError{Port: port, cause: err}
		}
	}
	if len(ports) == 1 {
		return &PortConflictError{Port: ports[0], cause: err}
	}
	return &PortConflictError{cause: err}
}

func publishedPorts(compose []byte) []int {
	var document struct {
		Services map[string]struct {
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if yaml.Unmarshal(compose, &document) != nil {
		return nil
	}
	seen := map[int]bool{}
	for _, service := range document.Services {
		for _, value := range service.Ports {
			parts := strings.Split(value, ":")
			if len(parts) != 3 || parts[0] != "127.0.0.1" {
				continue
			}
			port, err := strconv.Atoi(parts[1])
			if err == nil && port >= 1 && port <= 65535 {
				seen[port] = true
			}
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	slices.Sort(ports)
	return ports
}

func errorMentionsPort(message string, port int) bool {
	needle := ":" + strconv.Itoa(port)
	for offset := 0; ; {
		index := strings.Index(message[offset:], needle)
		if index < 0 {
			return false
		}
		index += offset
		next := index + len(needle)
		if next == len(message) || message[next] < '0' || message[next] > '9' {
			return true
		}
		offset = next
	}
}

func likelyConnectionFailure(err error) bool {
	message := strings.ToLower(err.Error())
	for _, value := range []string{"connection", "timeout", "no such host", "proxy", "tls", "unauthorized", "denied", "pull"} {
		if strings.Contains(message, value) {
			return true
		}
	}
	return false
}

func registryHosts(compose []byte) []string {
	var document struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if yaml.Unmarshal(compose, &document) != nil {
		return []string{"コンテナレジストリ"}
	}
	seen := map[string]bool{}
	for _, service := range document.Services {
		image := service.Image
		name := strings.SplitN(image, "@", 2)[0]
		parts := strings.Split(name, "/")
		host := "registry-1.docker.io"
		if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost") {
			host = parts[0]
		}
		seen[host] = true
	}
	hosts := make([]string, 0, len(seen))
	for host := range seen {
		hosts = append(hosts, host)
	}
	slices.Sort(hosts)
	return hosts
}

type ingressEndpoint struct {
	service string
	port    int
}

func ingressEndpoints(compose []byte) []ingressEndpoint {
	var document struct {
		Services map[string]struct {
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if yaml.Unmarshal(compose, &document) != nil {
		return nil
	}
	var result []ingressEndpoint
	for service, definition := range document.Services {
		if !strings.HasPrefix(service, "reprokit_proxy_") {
			continue
		}
		for _, value := range definition.Ports {
			parts := strings.Split(value, ":")
			if len(parts) != 3 || parts[0] != "127.0.0.1" || parts[2] != "80" {
				continue
			}
			port, err := strconv.Atoi(parts[1])
			if err == nil && port >= 1 && port <= 65535 {
				result = append(result, ingressEndpoint{service: service, port: port})
			}
		}
	}
	slices.SortFunc(result, func(left, right ingressEndpoint) int {
		if left.service != right.service {
			return strings.Compare(left.service, right.service)
		}
		return left.port - right.port
	})
	return result
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
