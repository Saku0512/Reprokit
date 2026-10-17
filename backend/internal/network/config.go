// Package network は企業ネットワーク向けの一時的な接続設定を扱う。
// 認証情報はRunnerの永続状態やLockfileへ渡さない。
package network

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

// Configは実行中のGit/Dockerプロセスにのみ適用する接続設定である。
type Config struct {
	HTTPProxy           string
	HTTPSProxy          string
	NoProxy             []string
	RegistryCredentials []RegistryCredential
}

// RegistryCredentialは一つのコンテナレジストリに限った認証情報である。
type RegistryCredential struct {
	Registry string
	Username string
	Password string
}

// ConnectionErrorは秘密情報を含めずに、接続先と対処方法を利用者へ示す。
type ConnectionError struct {
	Operation   string
	Host        string
	Remediation string
	cause       error
}

func (err *ConnectionError) Error() string {
	return fmt.Sprintf("%sで%sへ接続できません。%s", err.Operation, err.Host, err.Remediation)
}

func (err *ConnectionError) Unwrap() error { return err.cause }

func NewConnectionError(operation, host, remediation string, cause error) error {
	return &ConnectionError{Operation: operation, Host: host, Remediation: remediation, cause: cause}
}

// Validateは設定を利用前に検査し、設定値そのものをエラーへ含めない。
func (config Config) Validate() error {
	if err := validateProxy(config.HTTPProxy); err != nil {
		return fmt.Errorf("HTTP proxy: %w", err)
	}
	if err := validateProxy(config.HTTPSProxy); err != nil {
		return fmt.Errorf("HTTPS proxy: %w", err)
	}
	for _, value := range config.NoProxy {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return errors.New("NO_PROXY entry is invalid")
		}
	}
	seen := map[string]bool{}
	for _, credential := range config.RegistryCredentials {
		if !validRegistry(credential.Registry) || credential.Username == "" || credential.Password == "" {
			return errors.New("registry credential is invalid")
		}
		key := strings.ToLower(credential.Registry)
		if seen[key] {
			return errors.New("duplicate registry credential")
		}
		seen[key] = true
	}
	return nil
}

func validateProxy(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("must be an http or https URL")
	}
	return nil
}

func validRegistry(value string) bool {
	if strings.ContainsAny(value, "/?#@\r\n") || value == "" {
		return false
	}
	parsed, err := url.Parse("https://" + value)
	return err == nil && parsed.Hostname() != ""
}

// Environmentは継承したプロキシ値を除去し、Configの値だけを子プロセスへ渡す。
func (config Config) Environment(base []string) ([]string, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(base)+6)
	for _, entry := range base {
		name, _, found := strings.Cut(entry, "=")
		if !found || proxyEnvironment(strings.ToUpper(name)) {
			continue
		}
		result = append(result, entry)
	}
	add := func(name, value string) {
		if value != "" {
			result = append(result, name+"="+value)
		}
	}
	add("HTTP_PROXY", config.HTTPProxy)
	add("http_proxy", config.HTTPProxy)
	add("HTTPS_PROXY", config.HTTPSProxy)
	add("https_proxy", config.HTTPSProxy)
	if len(config.NoProxy) > 0 {
		value := strings.Join(config.NoProxy, ",")
		add("NO_PROXY", value)
		add("no_proxy", value)
	}
	return result, nil
}

func proxyEnvironment(name string) bool {
	return name == "HTTP_PROXY" || name == "HTTPS_PROXY" || name == "NO_PROXY" || name == "ALL_PROXY"
}

// DockerConfigDirは認証情報を0600の一時ファイルに書き、呼び出し元が必ず削除する。
func (config Config) DockerConfigDir() (string, func(), error) {
	if err := config.Validate(); err != nil {
		return "", nil, err
	}
	if len(config.RegistryCredentials) == 0 {
		return "", func() {}, nil
	}
	directory, err := os.MkdirTemp("", "reprokit-docker-config-*")
	if err != nil {
		return "", nil, err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		os.RemoveAll(directory)
		return "", nil, err
	}
	auths := make(map[string]map[string]string, len(config.RegistryCredentials))
	for _, credential := range config.RegistryCredentials {
		auths[credential.Registry] = map[string]string{"auth": base64.StdEncoding.EncodeToString([]byte(credential.Username + ":" + credential.Password))}
	}
	contents, err := json.Marshal(struct {
		Auths map[string]map[string]string `json:"auths"`
	}{auths})
	if err != nil {
		os.RemoveAll(directory)
		return "", nil, err
	}
	if err := os.WriteFile(directory+string(os.PathSeparator)+"config.json", contents, 0o600); err != nil {
		os.RemoveAll(directory)
		return "", nil, err
	}
	return directory, func() { _ = os.RemoveAll(directory) }, nil
}

// RegistryHostsは接続失敗の表示に使うため、認証情報を含まないホスト名を返す。
func (config Config) RegistryHosts() []string {
	hosts := make([]string, 0, len(config.RegistryCredentials))
	for _, credential := range config.RegistryCredentials {
		hosts = append(hosts, credential.Registry)
	}
	sort.Strings(hosts)
	return hosts
}

// HostFromURLは接続エラー表示用にURLからホスト名だけを取り出す。
func HostFromURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return "不明な接続先"
	}
	return parsed.Hostname()
}
