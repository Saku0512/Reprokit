package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInspectComposeAcceptsAndNormalizesLoopbackPorts(t *testing.T) {
	inspection, err := InspectCompose(readFixture(t, "allowed-bare-port.yaml"))
	if err != nil {
		t.Fatalf("InspectCompose() error = %v", err)
	}
	if !strings.Contains(string(inspection.Normalized), "127.0.0.1:8080:80") {
		t.Fatalf("normalized Compose does not bind the port to loopback:\n%s", inspection.Normalized)
	}
}

func TestFileBrowserDockerHubDraftPassesGuardrails(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "images", "CVE-2026-54088", "docker-compose.review.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectComposeWithGuardrails(contents, Guardrails{MaxRuntimeSeconds: 1800, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "none"})
	if err != nil {
		t.Fatalf("inspect draft Compose: %v", err)
	}
	var document struct {
		Services map[string]struct {
			Image string   `yaml:"image"`
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(inspection.Normalized, &document); err != nil {
		t.Fatal(err)
	}
	for name, image := range map[string]string{
		"vulnerable": "docker.io/saku0512/reprokit_cve-2026-54088@sha256:cdf1d4e63e4630f0f473e7f9cfd381c20450e65a87b86bceb9c944883ff6cd95",
		"fixed":      "docker.io/saku0512/reprokit_cve-2026-54088@sha256:22164b8c14c41f5aa4556f21442af35bc460a7d5ff85e262579fd37d5e3205a5",
	} {
		service, ok := document.Services[name]
		if !ok || service.Image != image || len(service.Ports) != 0 {
			t.Fatalf("normalized service %q = %#v", name, service)
		}
	}
}

func TestInspectComposeRejectsUnsafeFixtures(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		service string
		setting string
		message string
	}{
		{"privileged", "reject-privileged.yaml", "app", "privileged", "remove privileged: true"},
		{"host network", "reject-host-network.yaml", "app", "network_mode", "project Bridge network"},
		{"host PID", "reject-host-pid.yaml", "app", "pid", "host PID or IPC namespaces"},
		{"host IPC", "reject-host-ipc.yaml", "app", "ipc", "host PID or IPC namespaces"},
		{"host bind mount", "reject-host-bind-mount.yaml", "app", "volumes", "remove host bind mounts"},
		{"Docker socket", "reject-docker-socket.yaml", "app", "volumes", "Docker socket mounts"},
		{"capability", "reject-capability.yaml", "app", "cap_add", "Linux capability changes"},
		{"device", "reject-device.yaml", "app", "devices", "host device access"},
		{"external port", "reject-external-port.yaml", "app", "ports", "127.0.0.1:HOST:CONTAINER"},
		{"build context", "reject-build-context.yaml", "app", "build", "build contexts and Dockerfiles"},
		{"external input", "reject-env-file.yaml", "app", "env_file", "external Compose inputs"},
		{"relative path escape", "reject-relative-path-escape.yaml", "app", "volumes", "declared named volume"},
		{"unknown setting", "reject-unknown-setting.yaml", "app", "security_opt", "documented v0.1 allowlist"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := InspectCompose(readFixture(t, test.fixture))
			if err == nil {
				t.Fatal("InspectCompose() unexpectedly accepted unsafe Compose")
			}
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("InspectCompose() error = %T %v, want *ValidationError", err, err)
			}
			message := err.Error()
			for _, expected := range []string{`service "` + test.service + `"`, `setting "` + test.setting + `"`, test.message} {
				if !strings.Contains(message, expected) {
					t.Errorf("error %q does not contain %q", message, expected)
				}
			}
		})
	}
}

func TestInspectComposeRejectsAliasesAndUnknownTopLevelOptions(t *testing.T) {
	for _, fixture := range []string{"reject-yaml-alias.yaml", "reject-unknown-top-level.yaml"} {
		t.Run(fixture, func(t *testing.T) {
			if err := ValidateCompose(readFixture(t, fixture)); err == nil {
				t.Fatal("ValidateCompose() unexpectedly accepted unsafe Compose")
			}
		})
	}
}

func TestInspectComposeWithGuardrailsInjectsLimitsAndIsolatedNetwork(t *testing.T) {
	inspection, err := InspectComposeWithGuardrails([]byte(`services:
  app:
    image: example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    ports:
      - "8080:80"
`), Guardrails{MaxRuntimeSeconds: 1800, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "none"})
	if err != nil {
		t.Fatal(err)
	}
	contents := string(inspection.Normalized)
	for _, expected := range []string{"127.0.0.1:8080:80", "reprokit_isolated", "internal: true", "cpus: \"1\"", "memory: 512M", "pids: 128"} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("normalized Compose does not contain %q:\n%s", expected, contents)
		}
	}
	if strings.Contains(contents, "storage_opt") {
		t.Fatalf("normalized Compose must not require filesystem-specific storage_opt:\n%s", contents)
	}
}

func TestInspectComposeWithGuardrailsMovesLoopbackPortsToRunnerIngressProxy(t *testing.T) {
	inspection, err := InspectComposeWithGuardrails([]byte(`services:
  app:
    image: example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    ports:
      - "8080:80"
      - "8081:8081"
`), Guardrails{MaxRuntimeSeconds: 1800, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "none"})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Services map[string]struct {
			Command   []string `yaml:"command"`
			DependsOn []string `yaml:"depends_on"`
			Image     string   `yaml:"image"`
			Networks  []string `yaml:"networks"`
			Ports     []string `yaml:"ports"`
		} `yaml:"services"`
		Networks map[string]struct {
			Internal bool `yaml:"internal"`
		} `yaml:"networks"`
	}
	if err := yaml.Unmarshal(inspection.Normalized, &document); err != nil {
		t.Fatal(err)
	}
	if ports := document.Services["app"].Ports; len(ports) != 0 {
		t.Fatalf("Lab service must not publish directly: %#v", ports)
	}
	for name, target := range map[string]string{
		"reprokit_proxy_app_1": "app:80",
		"reprokit_proxy_app_2": "app:8081",
	} {
		proxy, exists := document.Services[name]
		if !exists || proxy.Image != ingressProxyImage || len(proxy.DependsOn) != 1 || proxy.DependsOn[0] != "app" || len(proxy.Networks) != 2 || proxy.Networks[0] != ingressNetworkName || proxy.Networks[1] != isolatedNetworkName || len(proxy.Command) != 6 || proxy.Command[0] != "caddy" || proxy.Command[5] != target {
			t.Fatalf("Runner ingress proxy %q = %#v", name, proxy)
		}
	}
	if ports := document.Services["reprokit_proxy_app_2"].Ports; len(ports) != 1 || ports[0] != "127.0.0.1:8081:80" {
		t.Fatalf("proxy port mapping = %#v", ports)
	}
	if !document.Networks[isolatedNetworkName].Internal || document.Networks[ingressNetworkName].Internal {
		t.Fatalf("networks = %#v", document.Networks)
	}
}

func TestInspectComposeWithGuardrailsKeepsUnpublishedServiceIsolated(t *testing.T) {
	inspection, err := InspectComposeWithGuardrails([]byte(`services:
  worker:
    image: example/worker@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
`), Guardrails{MaxRuntimeSeconds: 1800, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(inspection.Normalized), ingressProxyPrefix) || !strings.Contains(string(inspection.Normalized), "reprokit_isolated") {
		t.Fatalf("unexpected unpublished service Compose:\n%s", inspection.Normalized)
	}
}

func TestGuardrailsRejectsEgressAndUnboundedValues(t *testing.T) {
	for _, guardrails := range []Guardrails{
		{MaxRuntimeSeconds: 59, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "none"},
		{MaxRuntimeSeconds: 1800, CPUCores: 1, MemoryMiB: 512, PIDs: 128, DiskMiB: 1024, Egress: "allow"},
	} {
		if err := guardrails.Validate(); err == nil {
			t.Fatalf("Validate() accepted %#v", guardrails)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", "compose", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return contents
}
