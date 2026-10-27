package lockfile

import (
	"strings"
	"testing"

	"github.com/jphacks/sd_2601/internal/lab"
	"gopkg.in/yaml.v3"
)

func TestFromLabRecordsImmutableSourceInLockfileShape(t *testing.T) {
	resolved := lab.ResolvedLab{
		Source: lab.Source{
			URL:          "https://github.com/Saku0512/Reprokit.git",
			Revision:     "a94285e057faee8c7232e100116dc750cf17e257",
			Subdirectory: "httpd/CVE-2021-41773",
		},
		Registry: lab.RegistryReference{
			Revision:     "1111111111111111111111111111111111111111",
			ManifestPath: "manifests/reprokit/httpd-cve-2021-41773.yaml",
		},
		Manifest: lab.ManifestReference{
			SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	materialized := lab.MaterializedSource{
		URL:           "https://github.com/Saku0512/Reprokit.git",
		Revision:      "a94285e057faee8c7232e100116dc750cf17e257",
		Subdirectory:  "httpd/CVE-2021-41773",
		ComposeSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Files: []lab.SourceFile{
			{
				Path:   "docker-compose.yml",
				SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
	}

	provenance, err := FromLab(resolved, materialized)
	if err != nil {
		t.Fatalf("FromLab() error = %v", err)
	}
	contents, err := yaml.Marshal(provenance)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, expected := range []string{
		`revision: "1111111111111111111111111111111111111111"`,
		"manifestPath: manifests/reprokit/httpd-cve-2021-41773.yaml",
		"url: https://github.com/Saku0512/Reprokit.git",
		"revision: a94285e057faee8c7232e100116dc750cf17e257",
		"subdirectory: httpd/CVE-2021-41773",
		"path: docker-compose.yml",
	} {
		if !strings.Contains(string(contents), expected) {
			t.Errorf("lockfile provenance does not contain %q:\n%s", expected, contents)
		}
	}
}

func TestFromLabRejectsMismatchedMaterializedRevision(t *testing.T) {
	resolved := lab.ResolvedLab{
		Source: lab.Source{
			URL:          "https://github.com/Saku0512/Reprokit.git",
			Revision:     "a94285e057faee8c7232e100116dc750cf17e257",
			Subdirectory: "httpd/CVE-2021-41773",
		},
	}
	materialized := lab.MaterializedSource{
		URL:          resolved.Source.URL,
		Revision:     "ffffffffffffffffffffffffffffffffffffffff",
		Subdirectory: resolved.Source.Subdirectory,
	}

	if _, err := FromLab(resolved, materialized); err == nil {
		t.Fatal("FromLab() accepted a different materialized revision")
	}
}
