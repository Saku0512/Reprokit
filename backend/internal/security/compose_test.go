package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	for _, expected := range []string{"127.0.0.1:8080:80", "reprokit_isolated", "internal: true", "cpus: \"1\"", "memory: 512M", "pids: 128", "size: 1024M"} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("normalized Compose does not contain %q:\n%s", expected, contents)
		}
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
