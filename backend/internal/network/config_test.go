package network

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentUsesOnlyExplicitProxySettings(t *testing.T) {
	config := Config{HTTPProxy: "http://proxy.example:8080", HTTPSProxy: "https://proxy.example:8443", NoProxy: []string{"localhost", ".corp.example"}}
	environment, err := config.Environment([]string{"PATH=/bin", "HTTP_PROXY=http://inherited.invalid", "ALL_PROXY=socks://inherited.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(environment, "\n")
	for _, want := range []string{"PATH=/bin", "HTTP_PROXY=http://proxy.example:8080", "HTTPS_PROXY=https://proxy.example:8443", "NO_PROXY=localhost,.corp.example"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("environment is missing %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, "inherited.invalid") {
		t.Fatalf("inherited proxy escaped explicit configuration: %s", joined)
	}
}

func TestDockerConfigDirUsesPrivateTemporaryFile(t *testing.T) {
	config := Config{RegistryCredentials: []RegistryCredential{{Registry: "registry.example", Username: "learner", Password: "secret"}}}
	directory, cleanup, err := config.DockerConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	info, err := os.Stat(directory)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions = %v, %v", info, err)
	}
	contents, err := os.ReadFile(filepath.Join(directory, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "secret") || !strings.Contains(string(contents), base64.StdEncoding.EncodeToString([]byte("learner:secret"))) {
		t.Fatalf("unexpected Docker auth config: %s", contents)
	}
	file, err := os.Stat(filepath.Join(directory, "config.json"))
	if err != nil || file.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions = %v, %v", file, err)
	}
	cleanup()
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("credential directory remains after cleanup: %v", err)
	}
}

func TestConfigRejectsUnsafeSettingsWithoutDisclosingSecrets(t *testing.T) {
	_, err := (Config{HTTPProxy: "ftp://proxy.example", RegistryCredentials: []RegistryCredential{{Registry: "registry.example", Username: "user", Password: "top-secret"}}}).Environment(nil)
	if err == nil || strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("unsafe setting error = %v", err)
	}
}
