package lab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRegistryRevision = "1111111111111111111111111111111111111111"

func TestResolverResolvesRepositoryRegistry(t *testing.T) {
	resolved, err := (Resolver{}).Resolve("../../../registry", testRegistryRevision, "reprokit/filebrowser-cve-2026-54088")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Source.URL != "https://github.com/Saku0512/Reprokit.git" || resolved.Source.Revision != "2fff65ce608251a8b25b88ae86f9b50deea1e10c" {
		t.Fatalf("Source.URL = %q", resolved.Source.URL)
	}
	if len(resolved.Manifest.Bytes) == 0 {
		t.Fatal("Manifest.Bytes is empty")
	}
}

func TestResolverResolvesFixtureLabsThroughTheSamePath(t *testing.T) {
	resolver := Resolver{}
	tests := []struct {
		name         string
		labID        string
		subdirectory string
	}{
		{
			name:         "first fixture",
			labID:        "reprokit/httpd-cve-2021-41773",
			subdirectory: "images/CVE-2026-54088",
		},
		{
			name:         "second fixture",
			labID:        "reprokit/filebrowser-cve-2026-54088",
			subdirectory: "images/CVE-2026-54088",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := resolver.Resolve("testdata/registry", testRegistryRevision, test.labID)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.Source.URL != "https://github.com/Saku0512/Reprokit.git" {
				t.Fatalf("Source.URL = %q", resolved.Source.URL)
			}
			if resolved.Source.Revision != "a94285e057faee8c7232e100116dc750cf17e257" {
				t.Fatalf("Source.Revision = %q", resolved.Source.Revision)
			}
			if resolved.Source.Subdirectory != test.subdirectory {
				t.Fatalf("Source.Subdirectory = %q, want %q", resolved.Source.Subdirectory, test.subdirectory)
			}
			if resolved.Registry.Revision != testRegistryRevision {
				t.Fatalf("Registry.Revision = %q", resolved.Registry.Revision)
			}
			if len(resolved.Manifest.SHA256) != 64 {
				t.Fatalf("Manifest.SHA256 = %q", resolved.Manifest.SHA256)
			}
		})
	}
}

func TestResolverRejectsManifestPathTraversal(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "index.json"), `{
  "registryVersion": 1,
  "cves": {"CVE-2021-41773": ["reprokit/httpd-cve-2021-41773"]},
  "labs": {"reprokit/httpd-cve-2021-41773": {"manifest": "../outside.yaml"}}
}`)

	_, err := (Resolver{}).Resolve(root, testRegistryRevision, "reprokit/httpd-cve-2021-41773")
	if err == nil || !strings.Contains(err.Error(), "unsafe Manifest path") {
		t.Fatalf("Resolve() error = %v, want unsafe Manifest path", err)
	}
}

func TestResolverRejectsManifestSymlink(t *testing.T) {
	root := t.TempDir()
	manifestDirectory := filepath.Join(root, "manifests")
	if err := os.MkdirAll(manifestDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	writeTestFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(manifestDirectory, "lab.yaml")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "index.json"), `{
  "registryVersion": 1,
  "cves": {"CVE-2021-41773": ["reprokit/httpd-cve-2021-41773"]},
  "labs": {"reprokit/httpd-cve-2021-41773": {"manifest": "manifests/lab.yaml"}}
}`)

	_, err := (Resolver{}).Resolve(root, testRegistryRevision, "reprokit/httpd-cve-2021-41773")
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Resolve() error = %v, want symbolic link rejection", err)
	}
}

func TestResolverRejectsMutableRevision(t *testing.T) {
	root := copyTestRegistry(t)
	manifestPath := filepath.Join(root, "manifests/reprokit/httpd-cve-2021-41773.yaml")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents), "a94285e057faee8c7232e100116dc750cf17e257", "master", 1))
	writeTestFile(t, manifestPath, string(contents))

	_, err = (Resolver{}).Resolve(root, testRegistryRevision, "reprokit/httpd-cve-2021-41773")
	if err == nil || !strings.Contains(err.Error(), "40-character lowercase commit SHA") {
		t.Fatalf("Resolve() error = %v, want fixed commit rejection", err)
	}
}

func TestResolverRejectsNonGitHubSource(t *testing.T) {
	root := copyTestRegistry(t)
	manifestPath := filepath.Join(root, "manifests/reprokit/httpd-cve-2021-41773.yaml")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	contents = []byte(strings.Replace(string(contents), "https://github.com/Saku0512/Reprokit.git", "https://127.0.0.1/Reprokit.git", 1))
	writeTestFile(t, manifestPath, string(contents))

	_, err = (Resolver{}).Resolve(root, testRegistryRevision, "reprokit/httpd-cve-2021-41773")
	if err == nil || !strings.Contains(err.Error(), "not an approved repository") {
		t.Fatalf("Resolve() error = %v, want source host rejection", err)
	}
}

func TestValidateSourceLimitsReprokitToApprovedDirectories(t *testing.T) {
	source := Source{Type: "git", URL: "https://github.com/Saku0512/Reprokit.git", Revision: strings.Repeat("a", 40), Subdirectory: "images/CVE-2026-54088"}
	if err := validateSource(source); err != nil {
		t.Fatalf("approved source rejected: %v", err)
	}
	for _, path := range []string{"images/CVE-2026-54088/../CVE-2026-71557", "images", "images/CVE-2021-41773"} {
		source.Subdirectory = path
		if err := validateSource(source); err == nil {
			t.Fatalf("unapproved source directory %q accepted", path)
		}
	}
	source.Subdirectory = "images/CVE-2026-71557"
	if err := validateSource(source); err != nil {
		t.Fatalf("approved go-git source rejected: %v", err)
	}
	source.Subdirectory = "images/CVE-2026-54088"
	source.Revision = "main"
	if err := validateSource(source); err == nil {
		t.Fatal("mutable source revision accepted")
	}
}

func TestValidateSourceRejectsExternalRepositories(t *testing.T) {
	for _, origin := range []string{"https://github.com/third-party/lab.git", "https://github.com/other/repository.git"} {
		source := Source{Type: "git", URL: origin, Revision: strings.Repeat("a", 40), Subdirectory: "images/CVE-2026-54088"}
		if err := validateSource(source); err == nil {
			t.Fatalf("unapproved source %q accepted", origin)
		}
	}
}

func TestResolverRejectsUnknownManifestField(t *testing.T) {
	root := copyTestRegistry(t)
	manifestPath := filepath.Join(root, "manifests/reprokit/httpd-cve-2021-41773.yaml")
	file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("unknown: true\n"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = (Resolver{}).Resolve(root, testRegistryRevision, "reprokit/httpd-cve-2021-41773")
	if err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("Resolve() error = %v, want unknown field rejection", err)
	}
}

func copyTestRegistry(t *testing.T) string {
	t.Helper()
	destination := t.TempDir()
	if err := os.CopyFS(destination, os.DirFS("testdata/registry")); err != nil {
		t.Fatal(err)
	}
	return destination
}

func writeTestFile(t *testing.T, target, contents string) {
	t.Helper()
	if err := os.WriteFile(target, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
